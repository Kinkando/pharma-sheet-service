package http

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/pkg/profile"
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
	route.GET("", handler.getWarehouses)
	route.GET("/detail", handler.getWarehouseDetails)
	route.GET("/:warehouseID", handler.getWarehouse)
	route.POST("", handler.createWarehouse)
	route.PATCH("/:warehouseID", handler.updateWarehouse)
	route.DELETE("/:warehouseID", handler.deleteWarehouse)
	route.POST("/:warehouseID/locker", handler.createWarehouseLocker)
	route.PATCH("/:warehouseID/locker/:lockerID", handler.updateWarehouseLocker)
	route.DELETE("/:warehouseID/locker/:lockerID", handler.deleteWarehouseLocker)
	route.GET("/:warehouseID/sync/medicine", handler.summarizeMedicineSyncData)
	route.PUT("/:warehouseID/sync/medicine", handler.syncMedicine)

	warehouseUserRoute := route.Group("/:warehouseID/user")
	warehouseUserRoute.GET("", handler.getWarehouseUsers)
	warehouseUserRoute.PATCH("/:userID/approve", handler.approveUser)
	warehouseUserRoute.PATCH("/:userID/reject", handler.rejectUser)
	warehouseUserRoute.POST("", handler.createWarehouseUser)
	warehouseUserRoute.POST("/join", handler.joinWarehouse)
	warehouseUserRoute.PUT("/:userID/:role", handler.updateWarehouseUser)
	warehouseUserRoute.DELETE("/:userID", handler.deleteWarehouseUser)
}

func (h *WarehouseHandler) getWarehouses(c echo.Context) error {
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

func (h *WarehouseHandler) getWarehouse(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.GetWarehouseRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	warehouses, err := h.warehouseService.GetWarehouse(ctx, req.WarehouseID)
	if err != nil {
		logger.Context(ctx).Error(err)
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "warehouse id not found"})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, warehouses)
}

func (h *WarehouseHandler) getWarehouseDetails(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.FilterWarehouseDetail
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	req.Pagination.AssignDefault()

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	warehouses, err := h.warehouseService.GetWarehouseDetails(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
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
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *WarehouseHandler) deleteWarehouse(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.DeleteWarehouseRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	err := h.warehouseService.DeleteWarehouse(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
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
		return err
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
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *WarehouseHandler) deleteWarehouseLocker(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.DeleteWarehouseLockerRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	err := h.warehouseService.DeleteWarehouseLocker(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *WarehouseHandler) summarizeMedicineSyncData(c echo.Context) error {
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

	data, err := h.warehouseService.SummarizeMedicineFromGoogleSheet(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return c.JSON(http.StatusOK, data)
}

func (h *WarehouseHandler) syncMedicine(c echo.Context) error {
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

	err := h.warehouseService.SyncMedicineFromGoogleSheet(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *WarehouseHandler) getWarehouseUsers(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.FilterWarehouseUser
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	req.AssignDefault()

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	data, err := h.warehouseService.GetWarehouseUsers(ctx, req.WarehouseID, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return c.JSON(http.StatusOK, data)
}

func (h *WarehouseHandler) joinWarehouse(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.JoinWarehouseRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	userProfile, err := profile.UseProfile(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, echo.Map{"error": err.Error()})
	}

	err = h.warehouseService.JoinWarehouse(ctx, req.WarehouseID, userProfile.UserID)
	if err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *WarehouseHandler) approveUser(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.ApprovalWarehouseUserRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	err := h.warehouseService.ApproveUser(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *WarehouseHandler) rejectUser(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.ApprovalWarehouseUserRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	err := h.warehouseService.RejectUser(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *WarehouseHandler) createWarehouseUser(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.CreateWarehouseUserRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	err := h.warehouseService.CreateWarehouseUser(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *WarehouseHandler) updateWarehouseUser(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.UpdateWarehouseUserRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	err := h.warehouseService.UpdateWarehouseUser(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *WarehouseHandler) deleteWarehouseUser(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.DeleteWarehouseUserRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	err := h.warehouseService.DeleteWarehouseUser(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return c.NoContent(http.StatusNoContent)
}
