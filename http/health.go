package http

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

type HealthzHandler struct {
	pgPool      *pgxpool.Pool
	redisClient *redis.Client
}

func NewHealthzHandler(e *echo.Echo, pgPool *pgxpool.Pool, redisClient *redis.Client) {
	healthzHandler := HealthzHandler{pgPool: pgPool, redisClient: redisClient}

	e.GET("/livez", healthzHandler.Livez)
	e.GET("/readyz", healthzHandler.Readyz)
}

func (hh *HealthzHandler) Livez(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}

func (hh *HealthzHandler) Readyz(c echo.Context) error {
	pgCtx, pgCtxCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer pgCtxCancel()
	if err := hh.pgPool.Ping(pgCtx); err != nil {
		logger.Context(c.Request().Context()).Error(err)
		return c.JSON(http.StatusServiceUnavailable, echo.Map{"error": err.Error()})
	}

	redisCtx, redisCtxCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer redisCtxCancel()
	if err := hh.redisClient.Ping(redisCtx).Err(); err != nil {
		logger.Context(c.Request().Context()).Error(err)
		return c.JSON(http.StatusServiceUnavailable, echo.Map{"error": err.Error()})
	}

	return c.NoContent(http.StatusOK)
}
