package main

import (
	"log"
	"time"
	_ "time/tzdata"

	"github.com/kinkando/pharma-sheet-service/config"
	"github.com/kinkando/pharma-sheet-service/pkg/database/postgresql"
	"github.com/kinkando/pharma-sheet-service/pkg/database/redis"
	"github.com/kinkando/pharma-sheet-service/pkg/envconfig"
	"github.com/kinkando/pharma-sheet-service/pkg/google"
	httpmiddleware "github.com/kinkando/pharma-sheet-service/pkg/http/middleware"
	httpserver "github.com/kinkando/pharma-sheet-service/pkg/http/server"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/labstack/echo/v4"
)

func main() {
	var cfg config.Config
	if err := envconfig.Parse(&cfg); err != nil {
		log.Fatal(err)
	}
	logger.New(cfg.App.Environment)
	defer logger.Sync()

	redisClient := redis.NewClient(
		redis.WithHost(cfg.Redis.Host),
		redis.WithPort(cfg.Redis.Port),
		redis.WithUsername(cfg.Redis.Username),
		redis.WithPassword(cfg.Redis.Password),
		redis.WithMaxConnLifetime(time.Duration(cfg.Redis.MaxConnLifetime)*time.Minute),
		redis.WithMaxOpenConns(cfg.Redis.MaxOpenConns),
		redis.WithMaxIdleConns(cfg.Redis.MaxIdleConns),
	)
	defer redis.Shutdown(redisClient)

	pgPool := postgresql.New(
		postgresql.WithHost(cfg.PostgreSQL.Host),
		postgresql.WithUsername(cfg.PostgreSQL.Username),
		postgresql.WithPassword(cfg.PostgreSQL.Password),
		postgresql.WithDBName(cfg.PostgreSQL.DBName),
		postgresql.WithMaxConnLifetime(time.Duration(cfg.PostgreSQL.MaxConnLifetime)*time.Minute),
		postgresql.WithMaxOpenConns(cfg.PostgreSQL.MaxOpenConns),
		postgresql.WithMaxIdleConns(cfg.PostgreSQL.MaxIdleConns),
	)
	defer postgresql.Shutdown(pgPool)

	cloudStorage := google.NewStorage([]byte(cfg.Google.FirebaseCredential), cfg.Google.Storage.BucketName, cfg.Google.Storage.ExpiredTime)
	defer cloudStorage.Shutdown()

	httpServer := httpserver.New(
		httpserver.WithPort(cfg.App.Port),
		httpserver.WithMiddlewares([]echo.MiddlewareFunc{
			httpmiddleware.RequestID,
			// httpmiddleware.NewProfileProvider(
			// 	cfg.App.JWTKey,
			// 	redisClient,
			// 	"POST /auth/token/verify",
			// 	"POST /auth/token/refresh",
			// 	"POST /auth/token/revoke",
			// ),
		}),
	)

	httpServer.ListenAndServe()
	httpServer.GracefulShutdown()
}
