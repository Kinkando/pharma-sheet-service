package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"

	"firebase.google.com/go/auth"
	"github.com/google/uuid"
	genmodel "github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/google"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/pkg/profile"
	"github.com/kinkando/pharma-sheet-service/repository"
	"github.com/labstack/echo/v4"
	"github.com/sourcegraph/conc/pool"
)

type Warehouse interface {
	GetWarehouse(ctx context.Context, warehouseID string) (model.Warehouse, error)
	GetWarehouses(ctx context.Context) ([]model.Warehouse, error)
	GetWarehouseDetails(ctx context.Context, filter model.FilterWarehouseDetail) (model.PagingWithMetadata[model.WarehouseDetail], error)
	CreateWarehouse(ctx context.Context, req model.CreateWarehouseRequest) (string, error)
	UpdateWarehouse(ctx context.Context, req model.UpdateWarehouseRequest) error
	DeleteWarehouse(ctx context.Context, warehouseID string, bypass ...bool) error

	CountWarehouseUserStatus(ctx context.Context, warehouseID string) (model.CountWarehouseUserStatus, error)
	GetWarehouseUsers(ctx context.Context, warehouseID string, filter model.FilterWarehouseUser) (model.PagingWithMetadata[model.WarehouseUser], error)
	CreateWarehouseUser(ctx context.Context, req model.CreateWarehouseUserRequest) error
	UpdateWarehouseUser(ctx context.Context, req model.UpdateWarehouseUserRequest) error
	DeleteWarehouseUser(ctx context.Context, req model.DeleteWarehouseUserRequest) error
	JoinWarehouse(ctx context.Context, warehouseID, userID string) error
	CancelJoinWarehouse(ctx context.Context, warehouseID, userID string) error
	LeaveWarehouse(ctx context.Context, warehouseID, userID string) error
	ApproveUser(ctx context.Context, req model.ApprovalWarehouseUserRequest) error
	RejectUser(ctx context.Context, req model.ApprovalWarehouseUserRequest) error

	SummarizeMedicineFromGoogleSheet(ctx context.Context, req model.GetSyncMedicineMetadataRequest) (model.SyncMedicineMetadata, error)
	SyncMedicineFromGoogleSheet(ctx context.Context, req model.SyncMedicineRequest) error
}

type warehouse struct {
	warehouseRepository repository.Warehouse
	userRepository      repository.User
	medicineRepository  repository.Medicine
	firebaseAuthen      *auth.Client
	storage             google.Storage
	drive               google.Drive
	sheet               google.Sheet
	isSyncUniqueByID    bool
}

func NewWarehouseService(
	warehouseRepository repository.Warehouse,
	userRepository repository.User,
	medicineRepository repository.Medicine,
	firebaseAuthen *auth.Client,
	storage google.Storage,
	drive google.Drive,
	sheet google.Sheet,
) Warehouse {
	return &warehouse{
		warehouseRepository: warehouseRepository,
		userRepository:      userRepository,
		medicineRepository:  medicineRepository,
		firebaseAuthen:      firebaseAuthen,
		storage:             storage,
		drive:               drive,
		sheet:               sheet,
		isSyncUniqueByID:    false,
	}
}

func (s *warehouse) GetWarehouse(ctx context.Context, warehouseID string) (model.Warehouse, error) {
	return s.warehouseRepository.GetWarehouse(ctx, warehouseID)
}

func (s *warehouse) GetWarehouses(ctx context.Context) ([]model.Warehouse, error) {
	return s.warehouseRepository.GetWarehouses(ctx)
}

func (s *warehouse) GetWarehouseDetails(ctx context.Context, filter model.FilterWarehouseDetail) (res model.PagingWithMetadata[model.WarehouseDetail], err error) {
	data, total, err := s.warehouseRepository.GetWarehouseDetails(ctx, filter)
	if err != nil {
		logger.Context(ctx).Error(err)
		return res, err
	}

	conc := pool.New().WithContext(ctx).WithMaxGoroutines(5).WithCancelOnError().WithFirstError()
	for index, warehouse := range data {
		index, warehouse := index, warehouse
		conc.Go(func(ctx context.Context) error {
			medicineHouses, err := s.medicineRepository.GetMedicineHouses(ctx, model.FilterMedicineHouse{WarehouseID: warehouse.WarehouseID})
			if err != nil {
				return err
			}
			data[index].TotalMedicine = uint64(len(medicineHouses))

			if filter.Group != model.MyWarehouse {
				result, err := s.GetWarehouseUsers(ctx, warehouse.WarehouseID, model.FilterWarehouseUser{
					Pagination: model.Pagination{Page: 1, Limit: 9999},
					Status:     genmodel.PharmaSheetApprovalStatus_Approved,
				})
				if err != nil {
					return err
				}
				data[index].Users = result.Data
			}

			return nil
		})
	}
	if err = conc.Wait(); err != nil {
		logger.Context(ctx).Error(err)
		return res, err
	}

	res = model.PaginationResponse(data, filter.Pagination, total)
	return res, nil
}

func (s *warehouse) CreateWarehouse(ctx context.Context, req model.CreateWarehouseRequest) (string, error) {
	return s.warehouseRepository.CreateWarehouse(ctx, model.Warehouse{
		Name:        req.WarehouseName,
		WarehouseID: req.WarehouseID,
	})
}

func (s *warehouse) UpdateWarehouse(ctx context.Context, req model.UpdateWarehouseRequest) error {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.PharmaSheetRole_Admin, genmodel.PharmaSheetRole_Editor)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}
	err = s.warehouseRepository.UpdateWarehouse(ctx, model.Warehouse{
		WarehouseID: req.WarehouseID,
		Name:        req.WarehouseName,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return nil
}

func (s *warehouse) DeleteWarehouse(ctx context.Context, warehouseID string, bypass ...bool) error {
	if len(bypass) != 1 || !bypass[0] {
		err := s.checkWarehouseManagementRole(ctx, warehouseID, genmodel.PharmaSheetRole_Admin)
		if err != nil {
			logger.Context(ctx).Error(err)
			return err
		}
	}

	medicineBrands, err := s.medicineRepository.GetMedicineBrands(ctx, model.FilterMedicineBrand{WarehouseID: warehouseID})
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	deleteFile := func(ctx context.Context, fileID string) func(ctx context.Context) error {
		err = s.drive.Delete(ctx, fileID)
		if err != nil {
			logger.Context(ctx).Warn(err)
		}
		return nil
	}

	if len(medicineBrands) > 0 {
		conc := pool.New().WithContext(ctx).WithMaxGoroutines(5).WithCancelOnError()
		for _, brand := range medicineBrands {
			brand := brand
			if brand.BlisterImageURL != nil {
				conc.Go(deleteFile(ctx, *brand.BlisterImageURL))
			}
			if brand.BoxImageURL != nil {
				conc.Go(deleteFile(ctx, *brand.BoxImageURL))
			}
			if brand.TabletImageURL != nil {
				conc.Go(deleteFile(ctx, *brand.TabletImageURL))
			}
		}
		if err = conc.Wait(); err != nil {
			return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "medicine is not found"})
		}
	}

	err = s.warehouseRepository.DeleteWarehouse(ctx, warehouseID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": err.Error()})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return nil
}

func (s *warehouse) checkWarehouseManagementRole(ctx context.Context, warehouseID string, roles ...genmodel.PharmaSheetRole) (err error) {
	userProfile, err := profile.UseProfile(ctx)
	if err != nil {
		return
	}

	role, err := s.warehouseRepository.GetWarehouseRole(ctx, warehouseID, userProfile.UserID)
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	if !slices.Contains(roles, role) {
		return echo.NewHTTPError(http.StatusForbidden, echo.Map{"error": "permission denied"})
	}

	return nil
}

func (s *warehouse) CountWarehouseUserStatus(ctx context.Context, warehouseID string) (model.CountWarehouseUserStatus, error) {
	return s.warehouseRepository.CountWarehouseUserStatus(ctx, warehouseID)
}

func (s *warehouse) GetWarehouseUsers(ctx context.Context, warehouseID string, filter model.FilterWarehouseUser) (res model.PagingWithMetadata[model.WarehouseUser], err error) {
	data, total, err := s.warehouseRepository.GetWarehouseUsers(ctx, warehouseID, filter)
	if err != nil {
		logger.Context(ctx).Error(err)
		return res, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	conc := pool.New().WithContext(ctx).WithMaxGoroutines(5).WithCancelOnError()
	for index := range data {
		user, index := data[index], index
		if user.ImageURL != nil {
			conc.Go(func(ctx context.Context) error {
				url, err := s.storage.GetUrl(*user.ImageURL)
				if err != nil {
					logger.Context(ctx).Error(err)
					return err
				}
				data[index].ImageURL = &url

				return nil
			})
		}
	}
	if err = conc.Wait(); err != nil {
		logger.Context(ctx).Error(err)
		return res, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	res = model.PaginationResponse(data, filter.Pagination, total)
	return res, nil
}

func (s *warehouse) CreateWarehouseUser(ctx context.Context, req model.CreateWarehouseUserRequest) error {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.PharmaSheetRole_Admin)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	userReq := genmodel.PharmaSheetUsers{Email: req.Email}
	user, err := s.userRepository.GetUser(ctx, userReq)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Context(ctx).Error(err)
		return err
	}

	if errors.Is(err, sql.ErrNoRows) {
		userID, err := s.userRepository.CreateUser(ctx, userReq)
		if err != nil {
			logger.Context(ctx).Error(err)
			return err
		}
		user.UserID = uuid.MustParse(userID)
	}

	err = s.warehouseRepository.CreateWarehouseUser(ctx, req.WarehouseID, user.UserID.String(), req.Role, genmodel.PharmaSheetApprovalStatus_Approved)
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return nil
}

func (s *warehouse) UpdateWarehouseUser(ctx context.Context, req model.UpdateWarehouseUserRequest) error {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.PharmaSheetRole_Admin)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	userProfile, err := profile.UseProfile(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, echo.Map{"error": err.Error()})
	}

	if req.UserID == userProfile.UserID {
		return echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": "grant yourself is not allowed"})
	}

	err = s.warehouseRepository.UpdateWarehouseUser(ctx, genmodel.PharmaSheetWarehouseUsers{
		WarehouseID: req.WarehouseID,
		UserID:      uuid.MustParse(req.UserID),
		Role:        req.Role,
	})
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return nil
}

func (s *warehouse) DeleteWarehouseUser(ctx context.Context, req model.DeleteWarehouseUserRequest) error {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.PharmaSheetRole_Admin)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	userProfile, err := profile.UseProfile(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, echo.Map{"error": err.Error()})
	}

	if req.UserID == userProfile.UserID {
		return echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": "delete yourself is not allowed"})
	}

	err = s.warehouseRepository.DeleteWarehouseUser(ctx, req.WarehouseID, &req.UserID)
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return nil
}

func (s *warehouse) JoinWarehouse(ctx context.Context, warehouseID, userID string) error {
	return s.warehouseRepository.CreateWarehouseUser(ctx, warehouseID, userID, genmodel.PharmaSheetRole_Viewer, genmodel.PharmaSheetApprovalStatus_Pending)
}

func (s *warehouse) CancelJoinWarehouse(ctx context.Context, warehouseID, userID string) error {
	return s.warehouseRepository.DeleteWarehouseUser(ctx, warehouseID, &userID)
}

func (s *warehouse) LeaveWarehouse(ctx context.Context, warehouseID, userID string) error {
	err := s.warehouseRepository.DeleteWarehouseUser(ctx, warehouseID, &userID)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	_, total, err := s.warehouseRepository.GetWarehouseUsers(ctx, warehouseID, model.FilterWarehouseUser{
		Pagination: model.Pagination{Page: 1, Limit: 1},
		Role:       genmodel.PharmaSheetRole_Admin,
		Status:     genmodel.PharmaSheetApprovalStatus_Approved,
	})
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	if total == 0 {
		return s.DeleteWarehouse(ctx, warehouseID, true)
	}

	return nil
}

func (s *warehouse) ApproveUser(ctx context.Context, req model.ApprovalWarehouseUserRequest) error {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.PharmaSheetRole_Admin)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	userProfile, err := profile.UseProfile(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, echo.Map{"error": err.Error()})
	}

	if req.UserID == userProfile.UserID {
		return echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": "approve yourself is not allowed"})
	}

	status, err := s.warehouseRepository.GetWarehouseUserStatus(ctx, req.WarehouseID, req.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "userID is not found"})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	if status != genmodel.PharmaSheetApprovalStatus_Pending {
		return echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": "status is not pending"})
	}

	err = s.warehouseRepository.UpdateWarehouseUser(ctx, genmodel.PharmaSheetWarehouseUsers{
		WarehouseID: req.WarehouseID,
		UserID:      uuid.MustParse(req.UserID),
		Status:      genmodel.PharmaSheetApprovalStatus_Approved,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return nil
}

func (s *warehouse) RejectUser(ctx context.Context, req model.ApprovalWarehouseUserRequest) error {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.PharmaSheetRole_Admin)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	userProfile, err := profile.UseProfile(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, echo.Map{"error": err.Error()})
	}

	if req.UserID == userProfile.UserID {
		return echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": "reject yourself is not allowed"})
	}

	status, err := s.warehouseRepository.GetWarehouseUserStatus(ctx, req.WarehouseID, req.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "userID is not found"})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	if status != genmodel.PharmaSheetApprovalStatus_Pending {
		return echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": "status is not pending"})
	}

	err = s.warehouseRepository.DeleteWarehouseUser(ctx, req.WarehouseID, &req.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return nil
}

func (s *warehouse) SummarizeMedicineFromGoogleSheet(ctx context.Context, req model.GetSyncMedicineMetadataRequest) (metadata model.SyncMedicineMetadata, err error) {
	panic("TODO: implement me!")
	// data, err := s.getGoogleSheetData(ctx, model.SyncMedicineRequest(req), false)
	// if err != nil {
	// 	return
	// }

	// var (
	// 	totalMedicine        uint64 = 0
	// 	totalNewMedicine     uint64 = 0
	// 	totalUpdatedMedicine uint64 = 0
	// 	totalSkippedMedicine uint64 = 0
	// )

	// for _, medicineSheet := range data.MedicineSheets {
	// 	totalMedicine++

	// 	key := medicineSheet.Address
	// 	if s.isSyncUniqueByID {
	// 		key = medicineSheet.MedicationID
	// 	}

	// 	medicine, ok := data.MedicineData[key]
	// 	if !ok {
	// 		totalNewMedicine++
	// 		continue
	// 	}

	// 	if !ok || medicineSheet.IsDifferent(medicine, s.isSyncUniqueByID) {
	// 		totalUpdatedMedicine++
	// 	} else {
	// 		totalSkippedMedicine++
	// 	}
	// }

	// metadata = model.SyncMedicineMetadata{
	// 	Title:                data.SpreadsheetTitle,
	// 	SheetName:            data.Sheet.Properties.Title,
	// 	TotalMedicine:        totalMedicine,
	// 	TotalNewMedicine:     totalNewMedicine,
	// 	TotalUpdatedMedicine: totalUpdatedMedicine,
	// 	TotalSkippedMedicine: totalSkippedMedicine,
	// }

	// return metadata, nil
}

func (s *warehouse) SyncMedicineFromGoogleSheet(ctx context.Context, req model.SyncMedicineRequest) error {
	panic("TODO: implement me!")
	// data, err := s.getGoogleSheetData(ctx, req, true)
	// if err != nil {
	// 	return err
	// }

	// sheet := data.Sheet
	// spreadsheetID := data.SpreadsheetID
	// medicineSheets := data.MedicineSheets
	// medicineMapping := data.MedicineData
	// lockerID := data.LockerID

	// columnIDIndex := 0
	// if s.isSyncUniqueByID {
	// 	columns, err := s.sheet.ReadColumns(ctx, sheet, option.WithGoogleSheetReadColumnIgnoreUserEnteredFormat(true))
	// 	if err != nil {
	// 		logger.Context(ctx).Error(err)
	// 		return echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": err.Error()})
	// 	}

	// 	columnIDIndex = slices.IndexFunc(columns, func(column option.GoogleSheetUpdateColumn) bool {
	// 		return column.Value == "รหัส"
	// 	})
	// 	isFoundColumnID := columnIDIndex != -1
	// 	if !isFoundColumnID {
	// 		columnIDIndex = len(columns)
	// 		err = s.sheet.Update(
	// 			ctx,
	// 			spreadsheetID,
	// 			option.WithGoogleSheetUpdateSheetID(sheet.Properties.SheetId),
	// 			option.WithGoogleSheetUpdateSheetTitle(sheet.Properties.Title),
	// 			option.WithGoogleSheetUpdateColumns([]option.GoogleSheetUpdateColumn{{Value: "รหัส", Width: 500}}),
	// 			option.WithGoogleSheetUpdateColumnStartIndex(int64(columnIDIndex)+1),
	// 			option.WithGoogleSheetUpdateIsTextWraping(true),
	// 			option.WithGoogleSheetUpdateFontSize(20),
	// 		)
	// 		if err != nil {
	// 			logger.Context(ctx).Error(err)
	// 			return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	// 		}
	// 	}
	// }

	// for index, medicineSheet := range medicineSheets {
	// 	locker, ok := lockerID[medicineSheet.LockerName]
	// 	if !ok {
	// 		lockerID[medicineSheet.LockerName], err = s.lockerRepository.CreateLocker(ctx, genmodel.PharmaSheetLockers{
	// 			WarehouseID: uuid.MustParse(req.WarehouseID),
	// 			Name:        medicineSheet.LockerName,
	// 		})
	// 		if err != nil {
	// 			logger.Context(ctx).Error(err)
	// 			return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	// 		}
	// 		locker = lockerID[medicineSheet.LockerName]
	// 	}

	// 	createData := model.CreateMedicineRequest{
	// 		WarehouseID: req.WarehouseID,
	// 		LockerID:    locker,
	// 		Floor:       medicineSheet.Floor,
	// 		No:          medicineSheet.No,
	// 		Address:     medicineSheet.Address,
	// 		Description: medicineSheet.Description,
	// 		MedicalName: medicineSheet.MedicalName,
	// 		Label:       medicineSheet.Label,
	// 	}

	// 	updateData := model.UpdateMedicineRequest{
	// 		MedicationID: medicineSheet.MedicationID,
	// 		LockerID:     locker,
	// 		Floor:        medicineSheet.Floor,
	// 		No:           medicineSheet.No,
	// 		Address:      medicineSheet.Address,
	// 		Description:  medicineSheet.Description,
	// 		MedicalName:  medicineSheet.MedicalName,
	// 		Label:        medicineSheet.Label,
	// 	}

	// 	key := medicineSheet.Address
	// 	if s.isSyncUniqueByID {
	// 		key = medicineSheet.MedicationID
	// 	}

	// 	medicine, ok := medicineMapping[key]
	// 	if ok && locker == medicine.LockerID && !medicineSheet.IsDifferent(medicine, s.isSyncUniqueByID) {
	// 		continue
	// 	}

	// 	if ok {
	// 		updateData.MedicationID = medicine.MedicationID
	// 		err = s.medicineRepository.UpdateMedicine(ctx, updateData)
	// 		if err != nil {
	// 			logger.Context(ctx).Error(err)
	// 		}
	// 		continue
	// 	}

	// 	medicationID, err := s.medicineRepository.CreateMedicine(ctx, createData)
	// 	if err != nil {
	// 		logger.Context(ctx).Error(err)
	// 		continue
	// 	}

	// 	if !s.isSyncUniqueByID {
	// 		continue
	// 	}

	// 	col, _ := excelize.ColumnNumberToName(columnIDIndex + 1)
	// 	err = s.sheet.Update(
	// 		ctx,
	// 		spreadsheetID,
	// 		option.WithGoogleSheetUpdateSheetID(sheet.Properties.SheetId),
	// 		option.WithGoogleSheetUpdateSheetTitle(sheet.Properties.Title),
	// 		option.WithGoogleSheetUpdateData([][]option.GoogleSheetUpdateData{{{Value: medicationID}}}),
	// 		option.WithGoogleSheetUpdateIsTextWraping(true),
	// 		option.WithGoogleSheetUpdateFontSize(20),
	// 		option.WithGoogleSheetUpdateStartCellRange(fmt.Sprintf("%s%d", col, index+2)),
	// 	)
	// 	if err != nil {
	// 		logger.Context(ctx).Error(err)
	// 		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	// 	}
	// }
	// return nil
}

func (s *warehouse) getGoogleSheetData(ctx context.Context, req model.SyncMedicineRequest, isUpdateWarehouseSheet bool) (data model.GoogleSheetData, err error) {
	panic("TODO: implement me!")
	// err = s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.PharmaSheetRole_Admin, genmodel.PharmaSheetRole_Editor)
	// if err != nil {
	// 	logger.Context(ctx).Error(err)
	// 	return data, err
	// }

	// spreadsheetID, sheetID, err := extractSpreadsheetInfo(req.URL)
	// if err != nil {
	// 	logger.Context(ctx).Error(err)
	// 	return data, echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": "url is invalid"})
	// }

	// spreadsheet, err := s.sheet.Get(ctx, spreadsheetID)
	// if err != nil {
	// 	logger.Context(ctx).Error(err)
	// 	return data, echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "spreadsheetID is not found"})
	// }

	// var sheet *sheets.Sheet
	// for _, spreadSheet := range spreadsheet.Sheets {
	// 	if spreadSheet.Properties.SheetId == int64(sheetID) {
	// 		sheet = spreadSheet
	// 		break
	// 	}
	// }
	// if sheet == nil {
	// 	logger.Context(ctx).Warnf("sheetID is not found: %d", sheetID)
	// 	return data, echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "sheetID is not found"})
	// }

	// isConflict, err := s.warehouseRepository.CheckConflictWarehouseSheet(ctx, req.WarehouseID, spreadsheetID, sheetID)
	// if err != nil {
	// 	logger.Context(ctx).Error(err)
	// 	return data, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	// }
	// if isConflict {
	// 	return data, echo.NewHTTPError(http.StatusConflict, echo.Map{"error": "sheet is already sync by another warehouse"})
	// }

	// if isUpdateWarehouseSheet {
	// 	warehouseSheet := genmodel.PharmaSheetWarehouseSheets{
	// 		WarehouseID:   uuid.MustParse(req.WarehouseID),
	// 		SpreadsheetID: spreadsheetID,
	// 		SheetID:       sheetID,
	// 	}
	// 	err = s.warehouseRepository.UpsertWarehouseSheet(ctx, warehouseSheet)
	// 	if err != nil {
	// 		logger.Context(ctx).Error(err)
	// 		return data, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	// 	}
	// }

	// var medicineSheets []model.MedicineSheet
	// _, err = s.sheet.Read(ctx, sheet, &medicineSheets)
	// if err != nil {
	// 	logger.Context(ctx).Error(err)
	// 	return data, echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": err.Error()})
	// }

	// lockers, err := s.lockerRepository.GetLockers(ctx, req.WarehouseID)
	// if err != nil {
	// 	logger.Context(ctx).Error(err)
	// 	return data, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	// }
	// lockerID := make(map[string]string)
	// for _, locker := range lockers {
	// 	lockerID[locker.Name] = locker.LockerID.String()
	// }

	// medicines, err := s.medicineRepository.ListMedicines(ctx, model.ListMedicine{WarehouseID: req.WarehouseID})
	// if err != nil {
	// 	logger.Context(ctx).Error(err)
	// 	return data, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	// }
	// medicineMapping := make(map[string]model.Medicine)
	// for _, medicine := range medicines {
	// 	if s.isSyncUniqueByID {
	// 		medicineMapping[medicine.MedicationID] = medicine
	// 	} else {
	// 		medicineMapping[medicine.Address] = medicine
	// 	}
	// }

	// data = model.GoogleSheetData{
	// 	Sheet:            sheet,
	// 	SpreadsheetTitle: spreadsheet.Properties.Title,
	// 	SpreadsheetID:    spreadsheetID,
	// 	LockerID:         lockerID,
	// 	MedicineSheets:   medicineSheets,
	// 	MedicineData:     medicineMapping,
	// }

	// return data, nil
}

func extractSpreadsheetInfo(url string) (string, int32, error) {
	// Regular expressions for extracting the spreadsheet ID and gid
	spreadsheetIDPattern := `\/d\/([a-zA-Z0-9-_]+)`
	gidPattern := `gid=(\d+)`

	// Compile the regex patterns
	spreadsheetIDRegex, err := regexp.Compile(spreadsheetIDPattern)
	if err != nil {
		return "", 0, fmt.Errorf("failed to compile spreadsheet ID regex: %v", err)
	}

	gidRegex, err := regexp.Compile(gidPattern)
	if err != nil {
		return "", 0, fmt.Errorf("failed to compile gid regex: %v", err)
	}

	// Find the spreadsheet ID and gid using the regex
	spreadsheetIDMatches := spreadsheetIDRegex.FindStringSubmatch(url)
	if len(spreadsheetIDMatches) < 2 {
		return "", 0, fmt.Errorf("failed to extract spreadsheet ID from the URL")
	}

	gidMatches := gidRegex.FindStringSubmatch(url)
	if len(gidMatches) < 2 {
		return "", 0, fmt.Errorf("failed to extract gid from the URL")
	}

	gid, err := strconv.Atoi(gidMatches[1])
	if err != nil {
		return "", 0, fmt.Errorf("failed to convert gid to integer: %v", err)
	}

	// Return the extracted values
	return spreadsheetIDMatches[1], int32(gid), nil
}
