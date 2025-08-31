package config

import "time"

type AppConfig struct {
	Environment          string        `env:"ENVIRONMENT"`
	Port                 int           `env:"PORT"`
	JWTKey               string        `env:"JWT_KEY,required"`
	APIKey               string        `env:"API_KEY,required"`
	AccessTokenExpired   time.Duration `env:"ACCESS_TOKEN_EXPIRED,required"`
	RefreshTokenExpired  time.Duration `env:"REFRESH_TOKEN_EXPIRED,required"`
	Host                 string        `env:"HOST"`
	DisabledPingSelfHost bool          `env:"DISABLED_PING_SELF_HOST"`
}
