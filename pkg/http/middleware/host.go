package httpmiddleware

import (
	"context"

	"github.com/labstack/echo/v4"
)

func Host(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req := c.Request()
		ctx := req.Context()

		ctx = context.WithValue(ctx, "host", c.Scheme()+"://"+c.Request().Host)

		r := c.Request()
		*r = *r.WithContext(ctx)

		return next(c)
	}
}
