package config

type RedisConfig struct {
	Host     string `env:"HOST,required"`
	Port     int    `env:"PORT"`
	Username string `env:"USERNAME,required"`
	Password string `env:"PASSWORD,required"`
}
