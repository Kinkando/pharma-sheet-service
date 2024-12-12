package httpmiddleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/golang-jwt/jwt"
	"github.com/kinkando/pharma-sheet/pkg/profile"
	"github.com/labstack/echo/v4"
	"github.com/mitchellh/mapstructure"
	"github.com/redis/go-redis/v9"
)

const applicationPrefix = "PHARMA_SHEET"

func NewProfileProvider(jwtSecret string, client *redis.Client, skipMethodURLs ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			ctx := req.Context()

			for _, skipMethodURL := range skipMethodURLs {
				methodURL := req.Method + " " + req.URL.Path
				if skipMethodURL == methodURL {
					return next(c)
				}
			}

			authHeader := req.Header.Get(echo.HeaderAuthorization)
			m1 := regexp.MustCompile(`^Bearer `)
			tokenString := m1.ReplaceAllString(authHeader, "")

			var err error

			userProfile, sessionID, err := extractProfileFromJWT(tokenString, jwtSecret)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, echo.Map{"error": err.Error()})
			}
			ctx = context.WithValue(ctx, profile.ProfileKey, userProfile)

			if client != nil {
				key := fmt.Sprintf("%s:%s:%s:%s:%s", applicationPrefix, profile.AccessTokenPrefix, userProfile.Role, userProfile.UserID, sessionID)
				result, err := client.Exists(ctx, key).Result()
				if err != nil || result == 0 {
					return c.JSON(http.StatusUnauthorized, echo.Map{"error": "access token is not found"})
				}
			}

			r := c.Request()
			*r = *r.WithContext(ctx)

			return next(c)
		}
	}
}

func extractProfileFromJWT(tokenString string, jwtSecret string) (profile.Profile, string, error) {
	token, err := new(jwt.Parser).Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return profile.Profile{}, "", fmt.Errorf(`invalid JWT token: %v`, err)
	}

	if !token.Valid {
		return profile.Profile{}, "", errors.New("invalid JWT token")
	}

	var accessToken profile.AccessToken

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return profile.Profile{}, "", fmt.Errorf(`unable to map token to map claims: %v`, err)
	}
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return profile.Profile{}, "", fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	}
	err = mapstructure.Decode(claims, &accessToken)
	if err != nil {
		return profile.Profile{}, "", fmt.Errorf(`invalid token structure: %v`, err)
	}

	if accessToken.Type != profile.Access {
		return profile.Profile{}, "", fmt.Errorf(`unable to use refresh token: %v`, err)
	}

	profile := profile.Profile{
		UserID: accessToken.UserID,
		Role:   accessToken.Role,
	}
	return profile, accessToken.SessionID, nil
}
