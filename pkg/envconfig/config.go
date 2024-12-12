package envconfig

import (
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"github.com/labstack/gommon/log"
)

func Parse[T any](cfg T) error {
	if err := godotenv.Load(); err != nil {
		log.Warnf("unable to load configuration: %+v", err)
	}

	return env.Parse(&cfg)
}
