package service

import (
	"context"
	"net/http"
	"slices"

	"firebase.google.com/go/auth"
	"github.com/google/uuid"
	genmodel "github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/repository"
	"github.com/labstack/echo/v4"
	"github.com/sourcegraph/conc/pool"
)

type Warehouse interface {
	GetWarehouses(ctx context.Context) ([]model.Warehouse, error)
	CreateWarehouse(ctx context.Context, req model.CreateWarehouseRequest) (string, error)
	UpdateWarehouse(ctx context.Context, req model.UpdateWarehouseRequest) error
	CreateWarehouseLocker(ctx context.Context, req model.CreateWarehouseLockerRequest) (string, error)
	UpdateWarehouseLocker(ctx context.Context, req model.UpdateWarehouseLockerRequest) error

	GetWarehouseUsers(ctx context.Context, warehouseID string) ([]model.WarehouseUser, error)
}

type warehouse struct {
	warehouseRepository repository.Warehouse
	lockerRepository    repository.Locker
	firebaseAuthen      *auth.Client
}

func NewWarehouseService(
	warehouseRepository repository.Warehouse,
	lockerRepository repository.Locker,
	firebaseAuthen *auth.Client,
) Warehouse {
	return &warehouse{
		warehouseRepository: warehouseRepository,
		lockerRepository:    lockerRepository,
		firebaseAuthen:      firebaseAuthen,
	}
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

func (s *warehouse) checkWarehouseManagementRole(ctx context.Context, warehouseID string, roles ...genmodel.Role) (err error) {
	role, err := s.warehouseRepository.GetWarehouseRole(ctx, warehouseID)
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	if !slices.Contains(roles, role) {
		return echo.NewHTTPError(http.StatusForbidden, echo.Map{"error": "permission denied"})
	}

	return nil
}

func (s *warehouse) GetWarehouseUsers(ctx context.Context, warehouseID string) ([]model.WarehouseUser, error) {
	warehouseUsers, err := s.warehouseRepository.GetWarehouseUsers(ctx, warehouseID)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	conc := pool.New().WithContext(ctx).WithMaxGoroutines(5).WithCancelOnError()
	for index := range warehouseUsers {
		user, index := warehouseUsers[index], index
		if user.FirebaseUID != nil {
			conc.Go(func(ctx context.Context) error {
				authUser, err := s.firebaseAuthen.GetUser(ctx, *user.FirebaseUID)
				if err != nil {
					logger.Context(ctx).Error(err)
					return err
				}
				warehouseUsers[index].DisplayName = authUser.DisplayName
				warehouseUsers[index].ImageURL = authUser.PhotoURL

				return nil
			})
		}
	}
	if err = conc.Wait(); err != nil {
		logger.Context(ctx).Error(err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return warehouseUsers, nil
}
