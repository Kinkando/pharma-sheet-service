package http

import (
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/service"
	"github.com/labstack/echo/v4"
)

type MedicineHandler struct {
	medicineService service.Medicine
	validate        *validator.Validate
}

func NewMedicineHandler(e *echo.Echo, validate *validator.Validate, medicineService service.Medicine) {
	handler := &MedicineHandler{
		medicineService: medicineService,
		validate:        validate,
	}

	route := e.Group("/medicine")
	route.GET("", handler.getMedicines)
	route.GET("/master/all", handler.getAllMedicines)
	route.GET("/master/pagination", handler.getMedicineMasterPagination)
	route.GET("/:medicationID", handler.getMedicine)
	route.POST("/:medicationID", handler.createMedicine)
	route.PATCH("/:medicationID", handler.updateMedicine)
	route.DELETE("/:medicationID", handler.deleteMedicine)

	houseRoute := e.Group("/house")
	houseRoute.GET("", handler.getMedicineHouses)
	houseRoute.POST("", handler.createMedicineHouse)
	houseRoute.PUT("/:id", handler.updateMedicineHouse)
	houseRoute.DELETE("/:id", handler.deleteMedicineHouse)

	brandRoute := e.Group("/brand")
	brandRoute.GET("", handler.getMedicineBrands)
	brandRoute.POST("", handler.createMedicineBrand)
	brandRoute.PUT("/:id", handler.updateMedicineBrand)
	brandRoute.DELETE("/:id", handler.deleteMedicineBrand)

	historyRoute := e.Group("/history")
	historyRoute.POST("", handler.createMedicineBlisterDateHistory)
	historyRoute.DELETE("/:id", handler.deleteMedicineBlisterDateHistory)
}

func (h *MedicineHandler) getMedicines(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.FilterMedicine
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	req.Pagination.AssignDefault()

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	data, err := h.medicineService.GetMedicines(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, data)
}

func (h *MedicineHandler) getAllMedicines(c echo.Context) error {
	ctx := c.Request().Context()

	data, err := h.medicineService.ListMedicinesMaster(ctx)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, data)
}

func (h *MedicineHandler) getMedicineMasterPagination(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.Pagination
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	req.AssignDefault()

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	data, err := h.medicineService.GetMedicinesPagination(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, data)
}

func (h *MedicineHandler) getMedicine(c echo.Context) error {
	ctx := c.Request().Context()

	medicationID := c.Param("medicationID")

	medicine, err := h.medicineService.GetMedicine(ctx, medicationID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, medicine)
}

func (h *MedicineHandler) createMedicine(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.CreateMedicineRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	medicationID, err := h.medicineService.CreateMedicine(ctx, req)
	if err != nil {
		if model.IsConflictError(err) {
			return c.JSON(http.StatusConflict, echo.Map{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{"medicationID": medicationID})
}

func (h *MedicineHandler) updateMedicine(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.UpdateMedicineRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	err := h.medicineService.UpdateMedicine(ctx, req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *MedicineHandler) deleteMedicine(c echo.Context) error {
	ctx := c.Request().Context()

	medicationID := c.Param("medicationID")

	err := h.medicineService.DeleteMedicine(ctx, medicationID)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *MedicineHandler) getMedicineHouses(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.ListMedicineHouse
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	req.Pagination.AssignDefault()

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	data, err := h.medicineService.GetMedicineHouses(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, data)
}

func (h *MedicineHandler) createMedicineHouse(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.CreateMedicineHouseRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	id, err := h.medicineService.CreateMedicineHouse(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{"id": id})
}

func (h *MedicineHandler) updateMedicineHouse(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.UpdateMedicineHouseRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	err := h.medicineService.UpdateMedicineHouse(ctx, req)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *MedicineHandler) deleteMedicineHouse(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.DeleteMedicineHouseRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	_, err := h.medicineService.DeleteMedicineHouse(ctx, req.ID)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *MedicineHandler) getMedicineBrands(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.FilterMedicineWithBrand
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	req.Pagination.AssignDefault()

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	data, err := h.medicineService.GetMedicineBrands(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, data)
}

func (h *MedicineHandler) createMedicineBrand(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.CreateMedicineBrandRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if req.TradeName != nil && *req.TradeName == "" {
		req.TradeName = nil
	}
	if req.BlisterImageFile == nil && req.BoxImageFile == nil && req.TabletImageFile == nil && req.TradeName == nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "at least one of blisterImageFile, boxImageFile, tabletImageFile or tradeName must be provided"})
	}

	id, err := h.medicineService.CreateMedicineBrand(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{"id": id})
}

func (h *MedicineHandler) updateMedicineBrand(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.UpdateMedicineBrandRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if req.TradeName != nil && *req.TradeName == "" {
		req.TradeName = nil
	}
	if req.BlisterImageFile == nil && req.BoxImageFile == nil && req.TabletImageFile == nil && req.TradeName == nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "at least one of blisterImageFile, boxImageFile, tabletImageFile or tradeName must be provided"})
	}

	err := h.medicineService.UpdateMedicineBrand(ctx, req)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *MedicineHandler) deleteMedicineBrand(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.DeleteMedicineBrandRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	_, err := h.medicineService.DeleteMedicineBrand(ctx, req.ID)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *MedicineHandler) createMedicineBlisterDateHistory(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.CreateMedicineBlisterChangeDateHistoryRequest
	err := c.Bind(&req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	req.BlisterChangeDate, err = time.Parse(time.DateOnly, req.Date)
	if err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	id, err := h.medicineService.CreateMedicineBlisterChangeDateHistory(ctx, req)
	if err != nil {
		if model.IsConflictError(err) {
			return c.JSON(http.StatusConflict, echo.Map{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{"id": id})
}

func (h *MedicineHandler) deleteMedicineBlisterDateHistory(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.DeleteMedicineBlisterChangeDateHistoryRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	err := h.medicineService.DeleteMedicineBlisterChangeDateHistory(ctx, req.HistoryID)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}
