package main

import (
	"log"
	"time"
	_ "time/tzdata"

	"github.com/go-playground/validator/v10"
	"github.com/kinkando/pharma-sheet-service/config"
	"github.com/kinkando/pharma-sheet-service/http"
	"github.com/kinkando/pharma-sheet-service/pkg/database/postgresql"
	"github.com/kinkando/pharma-sheet-service/pkg/database/redis"
	"github.com/kinkando/pharma-sheet-service/pkg/envconfig"
	"github.com/kinkando/pharma-sheet-service/pkg/google"
	httpmiddleware "github.com/kinkando/pharma-sheet-service/pkg/http/middleware"
	httpserver "github.com/kinkando/pharma-sheet-service/pkg/http/server"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/pkg/option"
	"github.com/kinkando/pharma-sheet-service/repository"
	"github.com/kinkando/pharma-sheet-service/service"
	"github.com/labstack/echo/v4"
	"go.uber.org/ratelimit"
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

	googleDrive := google.NewGoogleDrive([]byte(cfg.Google.FirebaseCredential), cfg.Google.DriveRootFolderID)

	sheet := google.NewSheet(
		option.WithGoogleSheetClientCredentialJSON([]byte(cfg.Google.FirebaseCredential)),
		option.WithGoogleSheetClientRateLimiter(ratelimit.New(60, ratelimit.Per(time.Minute))), // but 20 requests per minute is ensured
	)

	firebaseAuthen := google.NewFirebaseAuthen([]byte(cfg.Google.FirebaseCredential))

	validate := validator.New()

	httpServer := httpserver.New(
		httpserver.WithPort(cfg.App.Port),
		httpserver.WithMiddlewares([]echo.MiddlewareFunc{
			httpmiddleware.RequestID,
			httpmiddleware.Host,
			httpmiddleware.NewProfileProvider(
				cfg.App.JWTKey,
				redisClient,
				"OPTION /",
				"HEAD /",
				"GET /livez",
				"GET /readyz",
				"POST /auth/token/verify",
				"POST /auth/token/refresh",
				"GET /file/*",
			),
		}),
	)

	userRepository := repository.NewUserRepository(pgPool)
	cacheRepository := repository.NewCacheRepository(redisClient, cfg.App.AccessTokenExpired, cfg.App.RefreshTokenExpired)
	warehouseRepository := repository.NewWarehouseRepository(pgPool)
	medicineRepository := repository.NewMedicineRepository(pgPool)

	jwtService := service.NewJWTService(cfg.App.JWTKey, cfg.App.AccessTokenExpired, cfg.App.RefreshTokenExpired)
	authenService := service.NewAuthenService(userRepository, cacheRepository, jwtService, firebaseAuthen)
	userService := service.NewUserService(userRepository, firebaseAuthen, cloudStorage)
	warehouseService := service.NewWarehouseService(warehouseRepository, userRepository, medicineRepository, cloudStorage)
	medicineService := service.NewMedicineService(medicineRepository, warehouseRepository, googleDrive)
	sheetService := service.NewSheetService(warehouseRepository, medicineRepository, googleDrive, sheet)

	http.NewHealthzHandler(httpServer.Routers(), pgPool, redisClient)
	http.NewDriveHandler(httpServer.Routers(), validate, googleDrive)
	http.NewAuthenHandler(httpServer.Routers(), validate, cfg.App.APIKey, authenService)
	http.NewUserHandler(httpServer.Routers(), validate, userService)
	http.NewWarehouseHandler(httpServer.Routers(), validate, warehouseService)
	http.NewMedicineHandler(httpServer.Routers(), validate, medicineService)
	http.NewSheetHandler(httpServer.Routers(), validate, sheetService)

	httpServer.ListenAndServe()
	httpServer.GracefulShutdown()
}
