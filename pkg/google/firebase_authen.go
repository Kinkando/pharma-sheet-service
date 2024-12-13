package google

import (
	"context"
	"time"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"github.com/labstack/gommon/log"
	"google.golang.org/api/option"
)

func NewFirebaseAuthen(credential []byte) *auth.Client {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	app, err := firebase.NewApp(ctx, &firebase.Config{}, option.WithCredentialsJSON(credential))
	if err != nil {
		log.Fatal(err)
	}

	auth, err := app.Auth(ctx)
	if err != nil {
		log.Fatal(err)
	}

	return auth
}
