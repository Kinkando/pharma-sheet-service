package httpmiddleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func ApiKey(apiKey string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			apiKeyReq := c.Request().Header.Get("X-API-Key")

			if apiKeyReq != apiKey {
				return c.JSON(http.StatusUnauthorized, echo.Map{"error": "api key is not found"})
			}

			return next(c)
		}
	}
}
