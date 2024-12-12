package google

import (
	"context"
	"time"

	firebase "firebase.google.com/go"
	"github.com/labstack/gommon/log"
	"google.golang.org/api/option"
)

func NewFirebaseAuthen(credential []byte) *firebase.App {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	app, err := firebase.NewApp(ctx, &firebase.Config{}, option.WithCredentialsJSON(credential))
	if err != nil {
		log.Fatal(err)
	}

	return app
}
