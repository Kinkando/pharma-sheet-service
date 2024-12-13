package httpmiddleware

import (
	"fmt"
	"net/http"

	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func ApiKey(client *redis.Client) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if client == nil {
				return next(c)
			}

			ctx := c.Request().Context()
			apiKey := c.Request().Header.Get("X-API-Key")

			key := fmt.Sprintf("API_KEY:" + apiKey)
			err := client.HGet(ctx, key, "platform").Err()
			if err != nil {
				logger.Context(ctx).Error(ctx, err)
				return c.JSON(http.StatusUnauthorized, echo.Map{"error": "api key is not found"})
			}

			return next(c)
		}
	}
}
