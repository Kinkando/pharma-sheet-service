package http

import (
	"net/http"
	"regexp"

	"github.com/go-playground/validator/v10"
	"github.com/kinkando/pharma-sheet-service/model"
	httpmiddleware "github.com/kinkando/pharma-sheet-service/pkg/http/middleware"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/service"
	"github.com/labstack/echo/v4"
)

type AuthenHandler struct {
	authenService service.Authen
	validate      *validator.Validate
}

func NewAuthenHandler(e *echo.Echo, validate *validator.Validate, apiKey string, authenService service.Authen) {
	handler := &AuthenHandler{
		authenService: authenService,
		validate:      validate,
	}

	route := e.Group("/auth")
	route.POST("/token/verify", handler.verifyToken, httpmiddleware.ApiKey(apiKey))
	route.POST("/token/refresh", handler.refreshToken, httpmiddleware.ApiKey(apiKey))
	route.POST("/token/revoke", handler.revokeToken)
}

func (h *AuthenHandler) verifyToken(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.VerifyTokenRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	jwt, err := h.authenService.VerifyToken(ctx, req.IDToken)
	if err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, jwt)
}

func (h *AuthenHandler) refreshToken(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.RefreshTokenRequest
	if err := c.Bind(&req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	jwt, err := h.authenService.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		logger.Context(ctx).Error(err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, jwt)
}

func (h *AuthenHandler) revokeToken(c echo.Context) error {
	ctx := c.Request().Context()

	authHeader := c.Request().Header.Get(echo.HeaderAuthorization)
	jwt := regexp.MustCompile(`^Bearer `).ReplaceAllString(authHeader, "")

	if err := h.authenService.RevokeToken(ctx, jwt); err != nil {
		logger.Error(ctx, err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}
