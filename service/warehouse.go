package service

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"

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
}

type warehouse struct {
	warehouseRepository repository.Warehouse
	userRepository      repository.User
	medicineRepository  repository.Medicine
	storage             google.Storage
}

func NewWarehouseService(
	warehouseRepository repository.Warehouse,
	userRepository repository.User,
	medicineRepository repository.Medicine,
	storage google.Storage,
) Warehouse {
	return &warehouse{
		warehouseRepository: warehouseRepository,
		userRepository:      userRepository,
		medicineRepository:  medicineRepository,
		storage:             storage,
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

	err := s.warehouseRepository.DeleteWarehouse(ctx, warehouseID)
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
