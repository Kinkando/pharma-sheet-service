package httpmiddleware

import (
	"context"

	"github.com/kinkando/pharma-sheet-service/pkg/generator"
	"github.com/labstack/echo/v4"
)

const (
	requestIDHeader string = "X-Request-ID"
)

func RequestID(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req, res, ctx := c.Request(), c.Response(), c.Request().Context()
		traceID := req.Header.Get(requestIDHeader)
		if traceID == "" {
			traceID = generator.UUID()
		}
		res.Header().Set(requestIDHeader, traceID)
		c.Set("requestID", traceID)

		ctx = context.WithValue(ctx, "requestID", traceID)
		*req = *req.WithContext(ctx)
		return next(c)
	}
}
