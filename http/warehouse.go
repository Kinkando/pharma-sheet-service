package http

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/service"
	"github.com/labstack/echo/v4"
)

type WarehouseHandler struct {
	warehouseService service.Warehouse
	validate         *validator.Validate
}

func NewWarehouseHandler(e *echo.Echo, validate *validator.Validate, warehouseService service.Warehouse) {
	handler := &WarehouseHandler{
		warehouseService: warehouseService,
		validate:         validate,
	}

	route := e.Group("/warehouse")
	route.GET("", handler.getWarehouse)
	route.POST("", handler.createWarehouse)
	route.PATCH("/:warehouseID", handler.updateWarehouse)
	route.POST("/:warehouseID/locker", handler.createWarehouseLocker)
	route.PATCH("/:warehouseID/locker/:lockerID", handler.updateWarehouseLocker)
}

func (h *WarehouseHandler) getWarehouse(c echo.Context) error {
	ctx := c.Request().Context()

	warehouses, err := h.warehouseService.GetWarehouses(ctx)
	if err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	if warehouses == nil {
		warehouses = []model.Warehouse{}
	}

	return c.JSON(http.StatusOK, warehouses)
}

func (h *WarehouseHandler) createWarehouse(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.CreateWarehouseRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	warehouseID, err := h.warehouseService.CreateWarehouse(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{"warehouseID": warehouseID})
}

func (h *WarehouseHandler) updateWarehouse(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.UpdateWarehouseRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	err := h.warehouseService.UpdateWarehouse(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *WarehouseHandler) createWarehouseLocker(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.CreateWarehouseLockerRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	lockerID, err := h.warehouseService.CreateWarehouseLocker(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{"lockerID": lockerID})
}

func (h *WarehouseHandler) updateWarehouseLocker(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.UpdateWarehouseLockerRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	err := h.warehouseService.UpdateWarehouseLocker(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}
