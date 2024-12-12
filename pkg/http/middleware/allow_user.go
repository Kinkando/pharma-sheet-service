package httpmiddleware

import (
	"net/http"

	"github.com/kinkando/pharma-sheet/pkg/profile"
	"github.com/labstack/echo/v4"
)

func AdminProfile(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		if _, err := profile.UseAdminProfile(ctx); err != nil {
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": err.Error()})
		}
		return next(c)
	}
}

func UserProfile(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		if _, err := profile.UseUserProfile(ctx); err != nil {
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": err.Error()})
		}
		return next(c)
	}
}
