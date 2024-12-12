package config

type RedisConfig struct {
	Host            string `env:"HOST,required"`
	Port            string `env:"PORT"`
	Username        string `env:"USERNAME,required"`
	Password        string `env:"PASSWORD,required"`
	MaxConnLifetime int    `env:"MAX_CONNECTION_LIFE_TIME"`
	MaxOpenConns    int    `env:"MAX_OPEN_CONNECTIONS"`
	MaxIdleConns    int    `env:"MAX_IDLE_CONNECTIONS"`
}
