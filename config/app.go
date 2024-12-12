package config

type AppConfig struct {
	Environment string `env:"ENVIRONMENT"`
	Port        int    `env:"PORT"`
	JWTKey      string `env:"JWT_KEY,required"`
}
