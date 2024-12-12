package config

type Config struct {
	App        AppConfig        `envPrefix:"APP_"`
	PostgreSQL PostgreSQLConfig `envPrefix:"POSTGRESQL_"`
}
