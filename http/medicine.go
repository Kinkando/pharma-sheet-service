package http

import (
	"net/http"

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
	route.GET("/:medicineID", handler.getMedicine)
	route.POST("", handler.createMedicine)
	route.PATCH("/:medicineID", handler.updateMedicine)
	route.DELETE("/:medicineID", handler.deleteMedicine)
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

func (h *MedicineHandler) getMedicine(c echo.Context) error {
	ctx := c.Request().Context()

	medicineID := c.Param("medicineID")

	medicine, err := h.medicineService.GetMedicine(ctx, medicineID)
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

	form, err := c.MultipartForm()
	if err != nil {
		logger.Error(ctx, err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	files := form.File["file"]
	if len(files) > 1 {
		logger.Error(ctx, "accept only 0-1 file")
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "accept only 0-1 file"})
	}
	if len(files) == 1 {
		req.File = files[0]
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	medicineID, err := h.medicineService.CreateMedicine(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{"medicineID": medicineID})
}

func (h *MedicineHandler) updateMedicine(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.UpdateMedicineRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	form, err := c.MultipartForm()
	if err != nil {
		logger.Error(ctx, err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	files := form.File["file"]
	if len(files) > 1 {
		logger.Error(ctx, "accept only 0-1 file")
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "accept only 0-1 file"})
	}
	if len(files) == 1 {
		req.File = files[0]
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	err = h.medicineService.UpdateMedicine(ctx, req)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *MedicineHandler) deleteMedicine(c echo.Context) error {
	ctx := c.Request().Context()

	medicineID := c.Param("medicineID")

	err := h.medicineService.DeleteMedicine(ctx, medicineID)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}
