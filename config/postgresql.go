package config

type PostgreSQLConfig struct {
	Host            string `env:"HOST,required"`
	Port            int    `env:"PORT"`
	Username        string `env:"USERNAME,required"`
	Password        string `env:"PASSWORD,required"`
	DBName          string `env:"DATABASE,required"`
	MaxConnLifetime int    `env:"MAX_CONNECTION_LIFE_TIME"`
	MaxOpenConns    int32  `env:"MAX_OPEN_CONNECTIONS"`
	MaxIdleConns    int32  `env:"MAX_IDLE_CONNECTIONS"`
}
