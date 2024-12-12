package postgresql

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/kinkando/pharma-sheet/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Option interface {
	apply(*postgreSQL)
}

type optionFunc func(*postgreSQL)

func (o optionFunc) apply(pgsql *postgreSQL) {
	o(pgsql)
}

func WithHost(host string) Option {
	return optionFunc(func(pgsql *postgreSQL) {
		pgsql.host = host
	})
}

func WithPort(port int) Option {
	return optionFunc(func(pgsql *postgreSQL) {
		pgsql.port = port
	})
}

func WithUsername(username string) Option {
	return optionFunc(func(pgsql *postgreSQL) {
		pgsql.username = username
	})
}

func WithPassword(password string) Option {
	return optionFunc(func(pgsql *postgreSQL) {
		pgsql.password = password
	})
}

func WithDBName(dbName string) Option {
	return optionFunc(func(pgsql *postgreSQL) {
		pgsql.dbName = dbName
	})
}

func WithMaxConnLifetime(d time.Duration) Option {
	return optionFunc(func(pgsql *postgreSQL) {
		pgsql.maxConnLifetime = d
	})
}

func WithMaxOpenConns(maxOpenConns int32) Option {
	return optionFunc(func(pgsql *postgreSQL) {
		pgsql.maxOpenConns = maxOpenConns
	})
}

func WithMaxIdleConns(maxIdleConns int32) Option {
	return optionFunc(func(pgsql *postgreSQL) {
		pgsql.maxIdleConns = maxIdleConns
	})
}

type postgreSQL struct {
	host                string
	port                int
	username            string
	password            string
	dbName              string
	queryString         map[string]string
	maxOpenConns        int32
	maxConnLifetime     time.Duration
	maxIdleConns        int32
	maxIdleConnLifetime time.Duration
}

func New(options ...Option) *pgxpool.Pool {
	pgsql := postgreSQL{maxConnLifetime: 15 * time.Minute, maxIdleConnLifetime: 15 * time.Minute}
	for _, o := range options {
		o.apply(&pgsql)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	host := pgsql.host
	if pgsql.port != 0 {
		host += ":" + strconv.Itoa(pgsql.port)
	}
	pgURL := fmt.Sprintf("postgres://%s:%s@%s/%s", pgsql.username, pgsql.password, host, pgsql.dbName)

	var qs string
	for k, v := range pgsql.queryString {
		qs += k + "=" + v + ","
	}
	if len(qs) > 0 {
		pgURL += "?" + qs[:len(qs)-1]
	}

	logger.Infof("postgresql: connecting to %s", pgURL)

	pgCfg, err := pgxpool.ParseConfig(pgURL)
	if err != nil {
		logger.Fatalf("postgresql: parse config: %s", err.Error())
	}
	pgCfg.ConnConfig.Config.ConnectTimeout = 10 * time.Second
	pgCfg.MaxConnLifetime = pgsql.maxConnLifetime
	pgCfg.MaxConns = pgsql.maxOpenConns
	if pgsql.maxIdleConns > pgsql.maxOpenConns {
		pgsql.maxIdleConns = pgsql.maxOpenConns
	}
	pgCfg.MaxConnIdleTime = pgsql.maxIdleConnLifetime
	pgCfg.MinConns = pgsql.maxIdleConns

	pgPool, err := pgxpool.NewWithConfig(ctx, pgCfg)
	if err != nil {
		logger.Fatalf("postgresql: connect: %s", err.Error())
	}

	if err = pgPool.Ping(ctx); err != nil {
		logger.Fatalf("postgresql: ping: %s", err.Error())
	}

	logger.Infof("postgresql: connected to %s:%d/%s", pgsql.host, pgsql.port, pgsql.dbName)
	return pgPool
}

func Shutdown(pgPool *pgxpool.Pool) {
	logger.Info("postgresql: shutting down")
	pgPool.Close()
	logger.Info("postgresql: shutdown")
}
