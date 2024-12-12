package config

type Config struct {
	App        AppConfig        `envPrefix:"APP_"`
	Redis      RedisConfig      `envPrefix:"REDIS_"`
	PostgreSQL PostgreSQLConfig `envPrefix:"POSTGRESQL_"`
}
