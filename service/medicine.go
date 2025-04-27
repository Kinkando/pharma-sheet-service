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
)

const (
	idTypeMedicine    = "MEDICINE"
	idTypeWarehouse   = "WAREHOUSE"
	medicineDirectory = "medicines"
)

type Medicine interface {
	GetMedicine(ctx context.Context, medicationID string) (model.Medicine, error)
	GetMedicines(ctx context.Context, filter model.FilterMedicine) (model.PagingWithMetadata[model.Medicine], error)
	CreateMedicine(ctx context.Context, req model.CreateMedicineRequest) (string, error)
	UpdateMedicine(ctx context.Context, req model.UpdateMedicineRequest) error
	DeleteMedicine(ctx context.Context, medicationID string) error

	CreateMedicineHouse(ctx context.Context, req model.CreateMedicineHouseRequest) (string, error)
	UpdateMedicineHouse(ctx context.Context, req model.UpdateMedicineHouseRequest) error
	DeleteMedicineHouse(ctx context.Context, id uuid.UUID) (int64, error)

	CreateMedicineBrand(ctx context.Context, req model.CreateMedicineBrandRequest) (string, error)
	UpdateMedicineBrand(ctx context.Context, req model.UpdateMedicineBrandRequest) error
	DeleteMedicineBrand(ctx context.Context, id uuid.UUID) (int64, error)

	CreateMedicineBlisterChangeDateHistory(ctx context.Context, req model.CreateMedicineBlisterChangeDateHistoryRequest) (string, error)
	DeleteMedicineBlisterChangeDateHistory(ctx context.Context, id uuid.UUID) error
}

type medicine struct {
	medicineRepository  repository.Medicine
	warehouseRepository repository.Warehouse
	storage             google.Drive
}

func NewMedicineService(
	medicineRepository repository.Medicine,
	warehouseRepository repository.Warehouse,
	storage google.Drive,
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
		data[index] = s.injectMedicineImageURL(ctx, data[index])
	}

	res = model.PaginationResponse(data, filter.Pagination, total)
	return res, nil
}

func (s *medicine) GetMedicine(ctx context.Context, medicationID string) (model.Medicine, error) {
	data, err := s.medicineRepository.GetMedicine(ctx, medicationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return data, echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "medicationID is not found"})
		}
		return data, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	data = s.injectMedicineImageURL(ctx, data)

	return data, nil
}

func (s *medicine) CreateMedicine(ctx context.Context, req model.CreateMedicineRequest) (string, error) {
	return s.medicineRepository.CreateMedicine(ctx, req)
}

func (s *medicine) UpdateMedicine(ctx context.Context, req model.UpdateMedicineRequest) error {
	return s.medicineRepository.UpdateMedicine(ctx, req)
}

func (s *medicine) DeleteMedicine(ctx context.Context, medicationID string) error {
	err := s.checkWarehouseManagementRole(ctx, medicationID, idTypeMedicine, genmodel.PharmaSheetRole_Admin, genmodel.PharmaSheetRole_Editor)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	medicineBrands, err := s.medicineRepository.GetMedicineBrands(ctx, model.FilterMedicineBrand{MedicationID: medicationID})
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	if len(medicineBrands) > 0 {
		for _, brand := range medicineBrands {
			if brand.BlisterImageURL != nil {
				err = s.storage.Delete(ctx, *brand.BlisterImageURL)
				if err != nil {
					logger.Context(ctx).Warn(err)
				}
			}

			if brand.BoxImageURL != nil {
				err = s.storage.Delete(ctx, *brand.BoxImageURL)
				if err != nil {
					logger.Context(ctx).Warn(err)
				}
			}

			if brand.TabletImageURL != nil {
				err = s.storage.Delete(ctx, *brand.TabletImageURL)
				if err != nil {
					logger.Context(ctx).Warn(err)
				}
			}
		}
	}

	rowsAffected, err := s.medicineRepository.DeleteMedicine(ctx, model.DeleteMedicineFilter{MedicationID: medicationID})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	if rowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "medicationID is not found"})
	}
	return nil
}

func (s *medicine) CreateMedicineHouse(ctx context.Context, req model.CreateMedicineHouseRequest) (string, error) {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, idTypeWarehouse, genmodel.PharmaSheetRole_Admin, genmodel.PharmaSheetRole_Editor)
	if err != nil {
		logger.Context(ctx).Error(err)
		return "", err
	}

	id, err := s.medicineRepository.CreateMedicineHouse(ctx, req)
	if err != nil {
		if model.IsConflictError(err) {
			return "", echo.NewHTTPError(http.StatusConflict, echo.Map{"error": err.Error()})
		}
		return "", echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return id, nil
}

func (s *medicine) UpdateMedicineHouse(ctx context.Context, req model.UpdateMedicineHouseRequest) error {
	houses, err := s.medicineRepository.GetMedicineHouses(ctx, model.FilterMedicineHouse{ID: req.ID})
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	if len(houses) == 0 {
		logger.Context(ctx).Errorf("houseID %s is not found", req.ID)
		return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "houseID is not found"})
	}
	err = s.checkWarehouseManagementRole(ctx, houses[0].WarehouseID, idTypeWarehouse, genmodel.PharmaSheetRole_Admin, genmodel.PharmaSheetRole_Editor)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	err = s.medicineRepository.UpdateMedicineHouse(ctx, req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return nil
}

func (s *medicine) DeleteMedicineHouse(ctx context.Context, id uuid.UUID) (int64, error) {
	houses, err := s.medicineRepository.GetMedicineHouses(ctx, model.FilterMedicineHouse{ID: id})
	if err != nil {
		logger.Context(ctx).Error(err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	if len(houses) == 0 {
		logger.Context(ctx).Errorf("houseID %s is not found", id.String())
		return 0, echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "houseID is not found"})
	}
	err = s.checkWarehouseManagementRole(ctx, houses[0].WarehouseID, idTypeWarehouse, genmodel.PharmaSheetRole_Admin, genmodel.PharmaSheetRole_Editor)
	if err != nil {
		logger.Context(ctx).Error(err)
		return 0, err
	}

	rowsAffected, err := s.medicineRepository.DeleteMedicineHouse(ctx, model.DeleteMedicineHouseFilter{ID: id})
	if err != nil {
		logger.Context(ctx).Error(err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	if rowsAffected == 0 {
		logger.Context(ctx).Errorf("houseID %s is not found", id.String())
		return 0, echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "houseID is not found"})
	}
	return rowsAffected, nil
}

func (s *medicine) CreateMedicineBrand(ctx context.Context, req model.CreateMedicineBrandRequest) (string, error) {
	if req.BlisterImageFile != nil {
		id, err := s.storage.UploadMultipart(ctx, medicineDirectory+"/"+req.MedicationID, req.BlisterImageFile)
		if err != nil {
			logger.Context(ctx).Error(err)
			return "", echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
		req.BlisterImageURL = &id
	}

	if req.TabletImageFile != nil {
		id, err := s.storage.UploadMultipart(ctx, medicineDirectory+"/"+req.MedicationID, req.TabletImageFile)
		if err != nil {
			logger.Context(ctx).Error(err)
			return "", echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
		req.TabletImageURL = &id
	}

	if req.BoxImageFile != nil {
		id, err := s.storage.UploadMultipart(ctx, medicineDirectory+"/"+req.MedicationID, req.BoxImageFile)
		if err != nil {
			logger.Context(ctx).Error(err)
			return "", echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
		req.BoxImageURL = &id
	}

	brandID, err := s.medicineRepository.CreateMedicineBrand(ctx, req)
	if err != nil {
		if model.IsConflictError(err) {
			return "", echo.NewHTTPError(http.StatusConflict, echo.Map{"error": err.Error()})
		}
		return "", echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return brandID, nil
}

func (s *medicine) UpdateMedicineBrand(ctx context.Context, req model.UpdateMedicineBrandRequest) error {
	brands, err := s.medicineRepository.GetMedicineBrands(ctx, model.FilterMedicineBrand{BrandID: req.BrandID})
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	if len(brands) == 0 {
		logger.Context(ctx).Errorf("brandID %s is not found", req.BrandID)
		return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "brandID is not found"})
	}
	brand := brands[0]

	if req.BlisterImageFile != nil {
		id, err := s.storage.UploadMultipart(ctx, medicineDirectory+"/"+brand.MedicationID, req.BlisterImageFile)
		if err != nil {
			logger.Context(ctx).Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
		req.DeleteBlisterImage = true
		req.BlisterImageURL = &id
	}

	if req.TabletImageFile != nil {
		id, err := s.storage.UploadMultipart(ctx, medicineDirectory+"/"+brand.MedicationID, req.TabletImageFile)
		if err != nil {
			logger.Context(ctx).Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
		req.DeleteTabletImage = true
		req.TabletImageURL = &id
	}

	if req.BoxImageFile != nil {
		id, err := s.storage.UploadMultipart(ctx, medicineDirectory+"/"+brand.MedicationID, req.BoxImageFile)
		if err != nil {
			logger.Context(ctx).Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
		req.DeleteBoxImage = true
		req.BoxImageURL = &id
	}

	if req.DeleteBlisterImage || req.DeleteTabletImage || req.DeleteBoxImage {
		if req.DeleteBlisterImage && brand.BlisterImageURL != nil {
			err = s.storage.Delete(ctx, *brand.BlisterImageURL)
			if err != nil {
				logger.Context(ctx).Warn(err)
			}
			if req.BlisterImageURL == nil {
				imageURL := "null"
				req.BlisterImageURL = &imageURL
			}
		}

		if req.DeleteBoxImage && brand.BoxImageURL != nil {
			err = s.storage.Delete(ctx, *brand.BoxImageURL)
			if err != nil {
				logger.Context(ctx).Warn(err)
			}
			if req.BoxImageURL == nil {
				imageURL := "null"
				req.BoxImageURL = &imageURL
			}
		}

		if req.DeleteTabletImage && brand.TabletImageURL != nil {
			err = s.storage.Delete(ctx, *brand.TabletImageURL)
			if err != nil {
				logger.Context(ctx).Warn(err)
			}
			if req.TabletImageURL == nil {
				imageURL := "null"
				req.TabletImageURL = &imageURL
			}
		}
	}

	err = s.medicineRepository.UpdateMedicineBrand(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return nil
}

func (s *medicine) DeleteMedicineBrand(ctx context.Context, id uuid.UUID) (int64, error) {
	medicineBrands, err := s.medicineRepository.GetMedicineBrands(ctx, model.FilterMedicineBrand{BrandID: id})
	if err != nil {
		logger.Context(ctx).Error(err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	if len(medicineBrands) > 0 {
		for _, brand := range medicineBrands {
			if brand.BlisterImageURL != nil {
				err = s.storage.Delete(ctx, *brand.BlisterImageURL)
				if err != nil {
					logger.Context(ctx).Warn(err)
				}
			}

			if brand.BoxImageURL != nil {
				err = s.storage.Delete(ctx, *brand.BoxImageURL)
				if err != nil {
					logger.Context(ctx).Warn(err)
				}
			}

			if brand.TabletImageURL != nil {
				err = s.storage.Delete(ctx, *brand.TabletImageURL)
				if err != nil {
					logger.Context(ctx).Warn(err)
				}
			}
		}
	}

	rowsAffected, err := s.medicineRepository.DeleteMedicineBrand(ctx, model.DeleteMedicineBrandFilter{BrandID: id})
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	if rowsAffected == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "medicationID is not found"})
	}
	return rowsAffected, nil
}

func (s *medicine) CreateMedicineBlisterChangeDateHistory(ctx context.Context, req model.CreateMedicineBlisterChangeDateHistoryRequest) (string, error) {
	err := s.checkWarehouseManagementRole(ctx, req.WarehouseID, idTypeWarehouse, genmodel.PharmaSheetRole_Admin, genmodel.PharmaSheetRole_Editor)
	if err != nil {
		logger.Context(ctx).Error(err)
		return "", err
	}

	id, err := s.medicineRepository.CreateMedicineBlisterChangeDateHistory(ctx, req)
	if err != nil {
		if model.IsConflictError(err) {
			return "", echo.NewHTTPError(http.StatusConflict, echo.Map{"error": err.Error()})
		}
		return "", echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return id, nil
}

func (s *medicine) DeleteMedicineBlisterChangeDateHistory(ctx context.Context, id uuid.UUID) error {
	blisterChangeDateHistory, err := s.medicineRepository.GetMedicineBlisterChangeDateHistory(ctx, id)
	if err != nil {
		logger.Context(ctx).Error(err)
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "blisterChangeDateHistoryID is not found"})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	err = s.checkWarehouseManagementRole(ctx, blisterChangeDateHistory.WarehouseID, idTypeWarehouse, genmodel.PharmaSheetRole_Admin, genmodel.PharmaSheetRole_Editor)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}
	err = s.medicineRepository.DeleteMedicineBlisterChangeDateHistory(ctx, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return nil
}

func (s *medicine) checkWarehouseManagementRole(ctx context.Context, id string, idType string, roles ...genmodel.PharmaSheetRole) (err error) {
	userProfile, err := profile.UseProfile(ctx)
	if err != nil {
		return
	}

	var role genmodel.PharmaSheetRole
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

func (s *medicine) injectMedicineImageURL(ctx context.Context, medicine model.Medicine) model.Medicine {
	host := ctx.Value("host").(string)
	for index, brand := range medicine.Brands {
		if brand.BlisterImageURL != nil {
			url := host + "/file/" + *brand.BlisterImageURL
			medicine.Brands[index].BlisterImageURL = &url
		}
		if brand.TabletImageURL != nil {
			url := host + "/file/" + *brand.TabletImageURL
			medicine.Brands[index].TabletImageURL = &url
		}
		if brand.BoxImageURL != nil {
			url := host + "/file/" + *brand.BoxImageURL
			medicine.Brands[index].BoxImageURL = &url
		}
	}
	return medicine
}
