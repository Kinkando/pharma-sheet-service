package http

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/service"
	"github.com/labstack/echo/v4"
)

type SheetHandler struct {
	sheetService service.Sheet
	validate     *validator.Validate
}

func NewSheetHandler(e *echo.Echo, validate *validator.Validate, sheetService service.Sheet) {
	handler := &SheetHandler{
		sheetService: sheetService,
		validate:     validate,
	}

	route := e.Group("/sheet")
	route.GET("/warehouse/:warehouseID", handler.summarizeMedicineSyncData)
	route.PUT("/warehouse/:warehouseID", handler.syncMedicine)
}

func (h *SheetHandler) summarizeMedicineSyncData(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.GetSyncMedicineMetadataRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	data, err := h.sheetService.SummarizeMedicineFromGoogleSheet(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return c.JSON(http.StatusOK, data)
}

func (h *SheetHandler) syncMedicine(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.SyncMedicineRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	err := h.sheetService.SyncMedicineFromGoogleSheet(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return c.NoContent(http.StatusNoContent)
}
