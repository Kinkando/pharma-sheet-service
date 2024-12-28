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
	"github.com/kinkando/pharma-sheet-service/pkg/option"
	"github.com/kinkando/pharma-sheet-service/pkg/profile"
	"github.com/kinkando/pharma-sheet-service/repository"
	"github.com/labstack/echo/v4"
	"github.com/sourcegraph/conc/pool"
	"github.com/xuri/excelize/v2"
	"google.golang.org/api/sheets/v4"
)

type Warehouse interface {
	GetWarehouse(ctx context.Context, warehouseID string) (model.Warehouse, error)
	GetWarehouses(ctx context.Context) ([]model.Warehouse, error)
	GetWarehouseDetails(ctx context.Context, filter model.FilterWarehouseDetail) (model.PagingWithMetadata[model.WarehouseDetail], error)
	CreateWarehouse(ctx context.Context, req model.CreateWarehouseRequest) (string, error)
	UpdateWarehouse(ctx context.Context, req model.UpdateWarehouseRequest) error
	DeleteWarehouse(ctx context.Context, req model.DeleteWarehouseRequest) error

	CreateWarehouseLocker(ctx context.Context, req model.CreateWarehouseLockerRequest) (string, error)
	UpdateWarehouseLocker(ctx context.Context, req model.UpdateWarehouseLockerRequest) error
	DeleteWarehouseLocker(ctx context.Context, req model.DeleteWarehouseLockerRequest) error

	GetWarehouseUsers(ctx context.Context, warehouseID string, filter model.FilterWarehouseUser) (model.PagingWithMetadata[model.WarehouseUser], error)
	CreateWarehouseUser(ctx context.Context, req model.CreateWarehouseUserRequest) error
	UpdateWarehouseUser(ctx context.Context, req model.UpdateWarehouseUserRequest) error
	DeleteWarehouseUser(ctx context.Context, req model.DeleteWarehouseUserRequest) error
	JoinWarehouse(ctx context.Context, warehouseID, userID string) error
	ApproveUser(ctx context.Context, req model.ApprovalWarehouseUserRequest) error
	RejectUser(ctx context.Context, req model.ApprovalWarehouseUserRequest) error

	SyncMedicineFromGoogleSheet(ctx context.Context, req model.SyncMedicineRequest) error
}

type warehouse struct {
	warehouseRepository repository.Warehouse
	lockerRepository    repository.Locker
	userRepository      repository.User
	medicineRepository  repository.Medicine
	firebaseAuthen      *auth.Client
	storage             google.Storage
	sheet               google.Sheet
	isSyncUniqueByID    bool
}

func NewWarehouseService(
	warehouseRepository repository.Warehouse,
	lockerRepository repository.Locker,
	userRepository repository.User,
	medicineRepository repository.Medicine,
	firebaseAuthen *auth.Client,
	storage google.Storage,
	sheet google.Sheet,
) Warehouse {
	return &warehouse{
		warehouseRepository: warehouseRepository,
		lockerRepository:    lockerRepository,
		userRepository:      userRepository,
		medicineRepository:  medicineRepository,
		firebaseAuthen:      firebaseAuthen,
		storage:             storage,
		sheet:               sheet,
		isSyncUniqueByID:    false,
	}
}

func (s *warehouse) GetWarehouse(ctx context.Context, warehouseID string) (model.Warehouse, error) {
	warehouse, err := s.warehouseRepository.GetWarehouse(ctx, warehouseID)
	if err != nil {
		return model.Warehouse{}, err
	}

	lockers, err := s.lockerRepository.GetLockers(ctx, warehouse.WarehouseID)
	if err != nil {
		return model.Warehouse{}, err
	}

	warehouseLockers := make([]model.Locker, 0, len(lockers))
	for _, locker := range lockers {
		warehouseLockers = append(warehouseLockers, model.Locker{
			LockerID:   locker.LockerID.String(),
			LockerName: locker.Name,
		})
	}
	warehouse.Lockers = warehouseLockers

	return warehouse, nil
}

func (s *warehouse) GetWarehouses(ctx context.Context) ([]model.Warehouse, error) {
	warehouses, err := s.warehouseRepository.GetWarehouses(ctx)
	if err != nil {
		return nil, err
	}

	for index, warehouse := range warehouses {
		lockers, err := s.lockerRepository.GetLockers(ctx, warehouse.WarehouseID)
		if err != nil {
			return nil, err
		}

		warehouseLockers := make([]model.Locker, 0, len(lockers))
		for _, locker := range lockers {
			warehouseLockers = append(warehouseLockers, model.Locker{
				LockerID:   locker.LockerID.String(),
				LockerName: locker.Name,
			})
		}
		warehouses[index].Lockers = warehouseLockers
	}

	return warehouses, nil
}

func (s *warehouse) GetWarehouseDetails(ctx context.Context, filter model.FilterWarehouseDetail) (res model.PagingWithMetadata[model.WarehouseDetail], err error) {
	data, total, err := s.warehouseRepository.GetWarehouseDetails(ctx, filter)
	if err != nil {
		logger.Context(ctx).Error(err)
		return res, err
	}

	for index, warehouse := range data {
		lockers, err := s.lockerRepository.GetLockers(ctx, warehouse.WarehouseID)
		if err != nil {
			return res, err
		}

		medicines, err := s.medicineRepository.ListMedicines(ctx, model.ListMedicine{WarehouseID: warehouse.WarehouseID})
		if err != nil {
			return res, err
		}

		warehouseLockers := make([]model.LockerDetail, 0, len(lockers))
		for _, locker := range lockers {
			var totalMedicine uint64 = 0
			for _, medicine := range medicines {
				if medicine.LockerID == locker.LockerID.String() {
					totalMedicine++
				}
			}
			warehouseLockers = append(warehouseLockers, model.LockerDetail{
				LockerID:      locker.LockerID.String(),
				LockerName:    locker.Name,
				TotalMedicine: totalMedicine,
			})
		}
		data[index].LockerDetails = warehouseLockers
		data[index].TotalLocker = uint64(len(lockers))
		data[index].TotalMedicine = uint64(len(medicines))
	}

	res = model.PaginationResponse(data, filter.Pagination, total)
	return res, nil
}

func (s *warehouse) CreateWarehouse(ctx context.Context, req model.CreateWarehouseRequest) (string, error) {
	return s.warehouseRepository.CreateWarehouse(ctx, model.Warehouse{
		Name: req.WarehouseName,
	})
}

func (s *warehouse) UpdateWarehouse(ctx context.Context, req model.UpdateWarehouseRequest) error {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.Role_Admin, genmodel.Role_Editor)
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

func (s *warehouse) DeleteWarehouse(ctx context.Context, req model.DeleteWarehouseRequest) error {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.Role_Admin)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	medicines, err := s.medicineRepository.ListMedicines(ctx, model.ListMedicine{WarehouseID: req.WarehouseID})
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	if len(medicines) > 0 {
		conc := pool.New().WithContext(ctx).WithMaxGoroutines(5).WithCancelOnError()
		for index := range medicines {
			medicine := medicines[index]
			if medicine.ImageURL != nil {
				conc.Go(func(ctx context.Context) error {
					err = s.storage.RemoveFile(ctx, *medicine.ImageURL)
					if err != nil {
						logger.Context(ctx).Warn(err)
					}
					return nil
				})
			}
		}
		if err = conc.Wait(); err != nil {
			return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "medicine is not found"})

		}

		rowsAffected, err := s.medicineRepository.DeleteMedicine(ctx, model.DeleteMedicineFilter{WarehouseID: req.WarehouseID})
		if err != nil {
			logger.Context(ctx).Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": "medicine is not found"})
		}

		if rowsAffected == 0 {
			return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "medicine is not found"})
		}
	}

	_, err = s.lockerRepository.DeleteLocker(ctx, model.DeleteLockerFilter{WarehouseID: req.WarehouseID})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	err = s.warehouseRepository.DeleteWarehouseUser(ctx, req.WarehouseID, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	s.warehouseRepository.DeleteWarehouseSheet(ctx, req.WarehouseID)

	err = s.warehouseRepository.DeleteWarehouse(ctx, req.WarehouseID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": err.Error()})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return nil
}

func (s *warehouse) CreateWarehouseLocker(ctx context.Context, req model.CreateWarehouseLockerRequest) (string, error) {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.Role_Admin, genmodel.Role_Editor)
	if err != nil {
		logger.Context(ctx).Error(err)
		return "", err
	}
	lockerID, err := s.lockerRepository.CreateLocker(ctx, genmodel.Lockers{
		WarehouseID: uuid.MustParse(req.WarehouseID),
		Name:        req.LockerName,
	})
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return lockerID, nil
}

func (s *warehouse) UpdateWarehouseLocker(ctx context.Context, req model.UpdateWarehouseLockerRequest) error {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.Role_Admin, genmodel.Role_Editor)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}
	err = s.lockerRepository.UpdateLocker(ctx, genmodel.Lockers{
		LockerID: uuid.MustParse(req.LockerID),
		Name:     req.LockerName,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return nil
}

func (s *warehouse) DeleteWarehouseLocker(ctx context.Context, req model.DeleteWarehouseLockerRequest) error {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.Role_Admin, genmodel.Role_Editor)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	medicines, err := s.medicineRepository.ListMedicines(ctx, model.ListMedicine{LockerID: req.LockerID})
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	if len(medicines) > 0 {
		conc := pool.New().WithContext(ctx).WithMaxGoroutines(5).WithCancelOnError()
		for index := range medicines {
			medicine := medicines[index]
			if medicine.ImageURL != nil {
				conc.Go(func(ctx context.Context) error {
					err = s.storage.RemoveFile(ctx, *medicine.ImageURL)
					if err != nil {
						logger.Context(ctx).Warn(err)
					}
					return nil
				})
			}
		}
		if err = conc.Wait(); err != nil {
			return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "medicine is not found"})

		}

		rowsAffected, err := s.medicineRepository.DeleteMedicine(ctx, model.DeleteMedicineFilter{LockerID: req.LockerID})
		if err != nil {
			logger.Context(ctx).Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": "medicine is not found"})
		}

		if rowsAffected == 0 {
			return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "medicine is not found"})
		}
	}

	rowsAffected, err := s.lockerRepository.DeleteLocker(ctx, model.DeleteLockerFilter{LockerID: req.LockerID})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	if rowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "lockerID is not found"})
	}

	return nil
}

func (s *warehouse) checkWarehouseManagementRole(ctx context.Context, warehouseID string, roles ...genmodel.Role) (err error) {
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
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.Role_Admin)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	userReq := genmodel.Users{Email: req.Email}
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

	err = s.warehouseRepository.CreateWarehouseUser(ctx, req.WarehouseID, user.UserID.String(), req.Role, genmodel.ApprovalStatus_Approved)
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return nil
}

func (s *warehouse) UpdateWarehouseUser(ctx context.Context, req model.UpdateWarehouseUserRequest) error {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.Role_Admin)
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

	err = s.warehouseRepository.UpdateWarehouseUser(ctx, genmodel.WarehouseUsers{
		WarehouseID: uuid.MustParse(req.WarehouseID),
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
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.Role_Admin)
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
	return s.warehouseRepository.CreateWarehouseUser(ctx, warehouseID, userID, genmodel.Role_Viewer, genmodel.ApprovalStatus_Pending)
}

func (s *warehouse) ApproveUser(ctx context.Context, req model.ApprovalWarehouseUserRequest) error {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.Role_Admin)
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

	if status != genmodel.ApprovalStatus_Pending {
		return echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": "status is not pending"})
	}

	err = s.warehouseRepository.UpdateWarehouseUser(ctx, genmodel.WarehouseUsers{
		WarehouseID: uuid.MustParse(req.WarehouseID),
		UserID:      uuid.MustParse(req.UserID),
		Status:      genmodel.ApprovalStatus_Approved,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return nil
}

func (s *warehouse) RejectUser(ctx context.Context, req model.ApprovalWarehouseUserRequest) error {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.Role_Admin)
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

	if status != genmodel.ApprovalStatus_Pending {
		return echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": "status is not pending"})
	}

	err = s.warehouseRepository.DeleteWarehouseUser(ctx, req.WarehouseID, &req.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return nil
}

func (s *warehouse) SyncMedicineFromGoogleSheet(ctx context.Context, req model.SyncMedicineRequest) error {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.Role_Admin, genmodel.Role_Editor)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	spreadsheetID, sheetID, err := extractSpreadsheetInfo(req.URL)
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": "url is invalid"})
	}

	spreadsheet, err := s.sheet.Get(ctx, spreadsheetID)
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "spreadsheetID is not found"})
	}

	var sheet *sheets.Sheet
	for _, spreadSheet := range spreadsheet.Sheets {
		if spreadSheet.Properties.SheetId == int64(sheetID) {
			sheet = spreadSheet
			break
		}
	}
	if sheet == nil {
		logger.Context(ctx).Warnf("sheetID is not found: %d", sheetID)
		return echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": "sheetID is not found"})
	}

	warehouseSheet := genmodel.WarehouseSheets{
		WarehouseID:   uuid.MustParse(req.WarehouseID),
		SpreadsheetID: spreadsheetID,
		SheetID:       sheetID,
	}
	err = s.warehouseRepository.UpsertWarehouseSheet(ctx, warehouseSheet)
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	var medicineSheets []model.MedicineSheet
	_, err = s.sheet.Read(ctx, sheet, &medicineSheets)
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	columnIDIndex := 0
	if s.isSyncUniqueByID {
		columns, err := s.sheet.ReadColumns(ctx, sheet, option.WithGoogleSheetReadColumnIgnoreUserEnteredFormat(true))
		if err != nil {
			logger.Context(ctx).Error(err)
			return echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": err.Error()})
		}

		columnIDIndex = slices.IndexFunc(columns, func(column option.GoogleSheetUpdateColumn) bool {
			return column.Value == "รหัส"
		})
		isFoundColumnID := columnIDIndex != -1
		if !isFoundColumnID {
			columnIDIndex = len(columns)
			err = s.sheet.Update(
				ctx,
				spreadsheetID,
				option.WithGoogleSheetUpdateSheetID(sheet.Properties.SheetId),
				option.WithGoogleSheetUpdateSheetTitle(sheet.Properties.Title),
				option.WithGoogleSheetUpdateColumns([]option.GoogleSheetUpdateColumn{{Value: "รหัส", Width: 500}}),
				option.WithGoogleSheetUpdateColumnStartIndex(int64(columnIDIndex)+1),
				option.WithGoogleSheetUpdateIsTextWraping(true),
				option.WithGoogleSheetUpdateFontSize(20),
			)
			if err != nil {
				logger.Context(ctx).Error(err)
				return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
			}
		}
	}

	lockers, err := s.lockerRepository.GetLockers(ctx, req.WarehouseID)
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	lockerID := make(map[string]string)
	for _, locker := range lockers {
		lockerID[locker.Name] = locker.LockerID.String()
	}

	medicines, err := s.medicineRepository.ListMedicines(ctx, model.ListMedicine{WarehouseID: req.WarehouseID})
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	medicineMapping := make(map[string]model.Medicine)
	for _, medicine := range medicines {
		if s.isSyncUniqueByID {
			medicineMapping[medicine.MedicineID] = medicine
		} else {
			medicineMapping[medicine.Address] = medicine
		}
	}

	for index, medicineSheet := range medicineSheets {
		locker, ok := lockerID[medicineSheet.LockerName]
		if !ok {
			lockerID[medicineSheet.LockerName], err = s.lockerRepository.CreateLocker(ctx, genmodel.Lockers{
				WarehouseID: uuid.MustParse(req.WarehouseID),
				Name:        medicineSheet.LockerName,
			})
			if err != nil {
				logger.Context(ctx).Error(err)
				return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
			}
			locker = lockerID[medicineSheet.LockerName]
		}

		if !s.isSyncUniqueByID {
			medicine, ok := medicineMapping[medicineSheet.Address]
			if ok && locker == medicine.LockerID && !medicineSheet.IsDifferent(medicine) {
				continue
			}

			if ok {
				err = s.medicineRepository.UpdateMedicine(ctx, model.UpdateMedicineRequest{
					MedicineID:  medicine.MedicineID,
					LockerID:    locker,
					Floor:       medicineSheet.Floor,
					No:          medicineSheet.No,
					Address:     medicineSheet.Address,
					Description: medicineSheet.Description,
					MedicalName: medicineSheet.MedicalName,
					Label:       medicineSheet.Label,
				})
				if err != nil {
					logger.Context(ctx).Error(err)
				}
			} else {
				_, err = s.medicineRepository.CreateMedicine(ctx, model.CreateMedicineRequest{
					WarehouseID: req.WarehouseID,
					LockerID:    locker,
					Floor:       medicineSheet.Floor,
					No:          medicineSheet.No,
					Address:     medicineSheet.Address,
					Description: medicineSheet.Description,
					MedicalName: medicineSheet.MedicalName,
					Label:       medicineSheet.Label,
				})
				if err != nil {
					logger.Context(ctx).Error(err)
				}
			}
			continue
		}

		if medicineSheet.MedicineID != "" {
			medicine, ok := medicineMapping[medicineSheet.MedicineID]
			if ok {
				if locker == medicine.LockerID && !medicineSheet.IsDifferent(medicine) {
					continue
				}
				err = s.medicineRepository.UpdateMedicine(ctx, model.UpdateMedicineRequest{
					MedicineID:  medicineSheet.MedicineID,
					LockerID:    locker,
					Floor:       medicineSheet.Floor,
					No:          medicineSheet.No,
					Address:     medicineSheet.Address,
					Description: medicineSheet.Description,
					MedicalName: medicineSheet.MedicalName,
					Label:       medicineSheet.Label,
				})
				if err != nil {
					logger.Context(ctx).Error(err)
				}
				continue
			}
		}

		req := model.CreateMedicineRequest{
			WarehouseID: req.WarehouseID,
			LockerID:    locker,
			Floor:       medicineSheet.Floor,
			No:          medicineSheet.No,
			Address:     medicineSheet.Address,
			Description: medicineSheet.Description,
			MedicalName: medicineSheet.MedicalName,
			Label:       medicineSheet.Label,
		}

		medicineID, err := s.medicineRepository.CreateMedicine(ctx, req)
		if err != nil {
			logger.Context(ctx).Error(err)
			continue
		}

		col, _ := excelize.ColumnNumberToName(columnIDIndex + 1)
		err = s.sheet.Update(
			ctx,
			spreadsheetID,
			option.WithGoogleSheetUpdateSheetID(sheet.Properties.SheetId),
			option.WithGoogleSheetUpdateSheetTitle(sheet.Properties.Title),
			option.WithGoogleSheetUpdateData([][]option.GoogleSheetUpdateData{{{Value: medicineID}}}),
			option.WithGoogleSheetUpdateIsTextWraping(true),
			option.WithGoogleSheetUpdateFontSize(20),
			option.WithGoogleSheetUpdateStartCellRange(fmt.Sprintf("%s%d", col, index+2)),
		)
		if err != nil {
			logger.Context(ctx).Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
	}
	return nil
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
