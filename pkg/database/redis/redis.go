package redis

import (
	"context"
	"time"

	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	goredis "github.com/redis/go-redis/v9"
)

const (
	defaultMaxRetries          = 3
	defaultMaxIdleConnLifetime = 30 * time.Minute
)

type Option interface {
	apply(*redis)
}

type optionFunc func(*redis)

func (o optionFunc) apply(r *redis) {
	o(r)
}

func WithHost(host string) Option {
	return optionFunc(func(r *redis) {
		r.host = host
	})
}

func WithPort(port string) Option {
	return optionFunc(func(r *redis) {
		r.port = port
	})
}

func WithUsername(username string) Option {
	return optionFunc(func(r *redis) {
		r.username = username
	})
}

func WithPassword(password string) Option {
	return optionFunc(func(r *redis) {
		r.password = password
	})
}

func WithDB(db int) Option {
	return optionFunc(func(r *redis) {
		r.db = db
	})
}

func WithMaxRetries(n int) Option {
	return optionFunc(func(r *redis) {
		r.maxRetries = n
	})
}

func WithMaxOpenConns(n int) Option {
	return optionFunc(func(r *redis) {
		r.maxOpenConns = n
	})
}

func WithMaxIdleConns(n int) Option {
	return optionFunc(func(r *redis) {
		r.maxRetries = n
	})
}

func WithMaxIdleConnLifetime(d time.Duration) Option {
	return optionFunc(func(r *redis) {
		r.maxIdleConnLifetime = d
	})
}

func WithMaxConnLifetime(d time.Duration) Option {
	return optionFunc(func(r *redis) {
		r.maxConnLifetime = d
	})
}

func WithOnConnect(onConnect func(context.Context, *goredis.Conn) error) Option {
	return optionFunc(func(r *redis) {
		r.onConnect = onConnect
	})
}

type redis struct {
	host                string
	port                string
	username            string
	password            string
	db                  int
	maxRetries          int
	readTimeout         time.Duration
	writeTimeout        time.Duration
	poolTimeout         time.Duration
	maxIdleConns        int
	maxOpenConns        int
	maxIdleConnLifetime time.Duration
	maxConnLifetime     time.Duration
	onConnect           func(context.Context, *goredis.Conn) error
}

func NewClient(options ...Option) *goredis.Client {
	r := &redis{
		maxRetries:          defaultMaxRetries,
		maxIdleConnLifetime: defaultMaxIdleConnLifetime,
	}
	for _, o := range options {
		o.apply(r)
	}

	logger.Infof("redis: connecting to %s:%s", r.host, r.port)

	client := goredis.NewClient(&goredis.Options{
		Addr:            r.host + ":" + r.port,
		OnConnect:       r.onConnect,
		Username:        r.username,
		Password:        r.password,
		DB:              r.db,
		MaxRetries:      r.maxRetries,
		MinIdleConns:    r.maxIdleConns,
		MaxIdleConns:    r.maxOpenConns,
		ConnMaxIdleTime: r.maxIdleConnLifetime,
		ConnMaxLifetime: r.maxConnLifetime,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		logger.Fatalf("redis: ping: %s", err.Error())
	}

	logger.Infof("redis: connected to %s:%s", r.host, r.port)
	return client
}

func Ping() func(context.Context, *goredis.Conn) error {
	return func(ctx context.Context, conn *goredis.Conn) error {
		return conn.Ping(ctx).Err()
	}
}

func Shutdown(r *goredis.Client) {
	logger.Info("redis: shutting down")
	if err := r.Close(); err != nil {
		logger.Errorf("redis: close: %s", err.Error())
		return
	}
	logger.Info("redis: shutdown")
}
