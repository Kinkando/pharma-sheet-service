package main

import (
	"log"
	"time"
	_ "time/tzdata"

	"github.com/kinkando/pharma-sheet/config"
	"github.com/kinkando/pharma-sheet/pkg/database/postgresql"
	"github.com/kinkando/pharma-sheet/pkg/envconfig"
	"github.com/kinkando/pharma-sheet/pkg/logger"
)

func main() {
	var cfg config.Config
	if err := envconfig.Parse(&cfg); err != nil {
		log.Fatal(err)
	}
	logger.New(cfg.App.Environment)
	defer logger.Sync()

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
}
