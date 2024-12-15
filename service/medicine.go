package service

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"

	genmodel "github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/google"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/pkg/profile"
	"github.com/kinkando/pharma-sheet-service/repository"
	"github.com/labstack/echo/v4"
)

const (
	idTypeMedicine    = "MEDICINE"
	idTypeWarehouse   = "WAREHOUSE"
	medicineDirectory = "medicines"
)

type Medicine interface {
	GetMedicine(ctx context.Context, medicineID string) (model.Medicine, error)
	GetMedicines(ctx context.Context, filter model.FilterMedicine) (model.PagingWithMetadata[model.Medicine], error)
	CreateMedicine(ctx context.Context, req model.CreateMedicineRequest) (string, error)
	UpdateMedicine(ctx context.Context, req model.UpdateMedicineRequest) error
	DeleteMedicine(ctx context.Context, medicineID string) error
}

type medicine struct {
	medicineRepository  repository.Medicine
	warehouseRepository repository.Warehouse
	storage             google.Storage
}

func NewMedicineService(
	medicineRepository repository.Medicine,
	warehouseRepository repository.Warehouse,
	storage google.Storage,
) Medicine {
	return &medicine{
		medicineRepository:  medicineRepository,
		warehouseRepository: warehouseRepository,
		storage:             storage,
	}
}

func (s *medicine) GetMedicines(ctx context.Context, filter model.FilterMedicine) (res model.PagingWithMetadata[model.Medicine], err error) {
	data, total, err := s.medicineRepository.GetMedicines(ctx, filter)
	if err != nil {
		logger.Context(ctx).Error(err)
		return res, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	for index := range data {
		if data[index].ImageURL != nil {
			url := s.storage.GetPublicUrl(*data[index].ImageURL)
			data[index].ImageURL = &url
		}
	}

	res = model.PaginationResponse(data, filter.Pagination, total)
	return res, nil
}

func (s *medicine) GetMedicine(ctx context.Context, medicineID string) (model.Medicine, error) {
	data, err := s.medicineRepository.GetMedicine(ctx, medicineID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return data, echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "medicineID is not found"})
		}
		return data, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	if data.ImageURL != nil {
		url := s.storage.GetPublicUrl(*data.ImageURL)
		data.ImageURL = &url
	}

	return data, nil
}

func (s *medicine) CreateMedicine(ctx context.Context, req model.CreateMedicineRequest) (string, error) {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, idTypeWarehouse, genmodel.Role_Admin, genmodel.Role_Editor)
	if err != nil {
		logger.Context(ctx).Error(err)
		return "", err
	}

	if req.File != nil {
		path, err := s.storage.UploadFile(ctx, req.File, medicineDirectory+"/"+req.WarehouseID)
		if err != nil {
			logger.Context(ctx).Error(err)
			return "", echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}

		err = s.storage.SetPublic(ctx, path)
		if err != nil {
			logger.Context(ctx).Error(err)
			return "", echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}

		req.ImageURL = &path
	}

	medicineID, err := s.medicineRepository.CreateMedicine(ctx, req)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return medicineID, nil
}

func (s *medicine) UpdateMedicine(ctx context.Context, req model.UpdateMedicineRequest) error {
	err := s.checkWarehouseManagementRole(ctx, req.MedicineID, idTypeMedicine, genmodel.Role_Admin, genmodel.Role_Editor)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	medicine, err := s.medicineRepository.GetMedicine(ctx, req.MedicineID)
	if err != nil {
		logger.Context(ctx).Error(err)
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "medicineID is not found"})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	if req.File != nil {
		path, err := s.storage.UploadFile(ctx, req.File, medicineDirectory+"/"+medicine.WarehouseID)
		if err != nil {
			logger.Context(ctx).Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}

		err = s.storage.SetPublic(ctx, path)
		if err != nil {
			logger.Context(ctx).Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}

		req.DeleteImage = true
		req.ImageURL = &path
	}

	if req.DeleteImage && medicine.ImageURL != nil {
		err = s.storage.RemoveFile(ctx, *medicine.ImageURL)
		if err != nil {
			logger.Context(ctx).Warn(err)
		}
		if req.ImageURL == nil {
			imageURL := "null"
			req.ImageURL = &imageURL
		}
	}

	err = s.medicineRepository.UpdateMedicine(ctx, req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return nil
}

func (s *medicine) DeleteMedicine(ctx context.Context, medicineID string) error {
	err := s.checkWarehouseManagementRole(ctx, medicineID, idTypeMedicine, genmodel.Role_Admin, genmodel.Role_Editor)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	medicine, err := s.medicineRepository.GetMedicine(ctx, medicineID)
	if err != nil {
		logger.Context(ctx).Error(err)
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "medicineID is not found"})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	if medicine.ImageURL != nil {
		err = s.storage.RemoveFile(ctx, *medicine.ImageURL)
		if err != nil {
			logger.Context(ctx).Warn(err)
		}
	}

	err = s.medicineRepository.DeleteMedicine(ctx, medicineID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return nil
}

func (s *medicine) checkWarehouseManagementRole(ctx context.Context, id string, idType string, roles ...genmodel.Role) (err error) {
	userProfile, err := profile.UseProfile(ctx)
	if err != nil {
		return
	}

	var role genmodel.Role
	switch idType {
	case idTypeMedicine:
		role, err = s.medicineRepository.GetMedicineRole(ctx, id, userProfile.UserID)
		if err != nil {
			logger.Context(ctx).Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
	case idTypeWarehouse:
		role, err = s.warehouseRepository.GetWarehouseRole(ctx, id, userProfile.UserID)
		if err != nil {
			logger.Context(ctx).Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
	default:
		return echo.NewHTTPError(http.StatusForbidden, echo.Map{"error": errors.ErrUnsupported})
	}

	if !slices.Contains(roles, role) {
		return echo.NewHTTPError(http.StatusForbidden, echo.Map{"error": "permission denied"})
	}

	return nil
}
