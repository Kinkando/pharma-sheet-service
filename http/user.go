package http

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/service"
	"github.com/labstack/echo/v4"
)

type UserHandler struct {
	userService service.User
	validate    *validator.Validate
}

func NewUserHandler(e *echo.Echo, validate *validator.Validate, userService service.User) {
	handler := &UserHandler{
		userService: userService,
		validate:    validate,
	}

	route := e.Group("/user")
	route.GET("", handler.getUser)
	route.PATCH("", handler.updateUser)
}

func (h *UserHandler) getUser(c echo.Context) error {
	ctx := c.Request().Context()

	user, err := h.userService.GetUserInfo(ctx)
	if err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, user)
}

func (h *UserHandler) updateUser(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	err := h.userService.UpdateUserInfo(ctx, req)
	if err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}
