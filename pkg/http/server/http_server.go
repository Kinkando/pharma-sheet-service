package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	validatorv10 "github.com/go-playground/validator/v10"
	"github.com/kinkando/pharma-sheet/pkg/logger"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// HTTPServer is the top-level wrapper instance of Echo.
type HTTPServer interface {
	ListenAndServe()
	GracefulShutdown()
	Routers() *echo.Echo
}

// Option wraps an apply method to bind optional arguments to HTTP server
type Option interface {
	apply(*httpServer)
}

type optionFunc func(*httpServer)

func (o optionFunc) apply(hs *httpServer) {
	o(hs)
}

// WithPort binds a port that will listen and serve an HTTP
func WithPort(port int) Option {
	return optionFunc(func(hs *httpServer) {
		hs.port = port
	})
}

// WithMiddlewares binds middlewares to HTTP server
func WithMiddlewares(middlewares []echo.MiddlewareFunc) Option {
	return optionFunc(func(hs *httpServer) {
		hs.middlewares = middlewares
	})
}

// WithCustomValidators binds custom tags to HTTP server
func WithCustomValidators(tags map[string]validatorv10.Func) Option {
	return optionFunc(func(hs *httpServer) {
		hs.customValidators = tags
	})
}

// WithCORSConfig binds CORS configuration to HTTP server
func WithCORSConfig(cors *CORSConfig) Option {
	return optionFunc(func(hs *httpServer) {
		hs.corsConfig = cors
		if len(hs.corsConfig.AllowOrigins) == 0 {
			hs.corsConfig.AllowOrigins = []string{"*"}
		}
	})
}

// WithLoggingSkipper binds logging skipper to HTTP server
func WithLoggingSkipper(skipper *LoggingSkipper) Option {
	return optionFunc(func(hs *httpServer) {
		hs.loggingSkipper = skipper
	})
}

type CORSConfig struct {
	AllowOrigins []string
	AllowHeaders []string
	AllowMethods []string
}

type httpServer struct {
	// port is the port that the server will listen and serve
	port int
	// router is the instance of Echo
	router *echo.Echo
	// middlewares is the list of middlewares that will be applied to the server
	middlewares []echo.MiddlewareFunc
	// if withValidator is true, then validatorCustomTags will be used
	customValidators map[string]validatorv10.Func
	// corsConfig is the CORS configuration of the server
	corsConfig *CORSConfig
	// loggingSkipper is the skipper for logging
	loggingSkipper *LoggingSkipper
}

type LoggingSkipper struct {
	LoggingSkipper     echomiddleware.Skipper
	BeforeNextSkippers []echomiddleware.Skipper
	LogValuesSkippers  []echomiddleware.Skipper
}

// New creates an instance of HTTPServer
func New(options ...Option) HTTPServer {
	hs := &httpServer{
		port: 3000,
		corsConfig: &CORSConfig{
			AllowOrigins: []string{"*"},
			AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
			AllowMethods: []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
		},
		loggingSkipper: &LoggingSkipper{
			LoggingSkipper:     HealthCheckSkipper,
			BeforeNextSkippers: make([]echomiddleware.Skipper, 0),
			LogValuesSkippers:  make([]echomiddleware.Skipper, 0),
		},
	}
	for _, o := range options {
		o.apply(hs)
	}

	e := echo.New()

	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: hs.corsConfig.AllowOrigins,
		AllowHeaders: hs.corsConfig.AllowHeaders,
		AllowMethods: hs.corsConfig.AllowMethods,
	}))
	e.Use(echomiddleware.GzipWithConfig(echomiddleware.GzipConfig{Level: 6}))
	e.Use(echomiddleware.RemoveTrailingSlash())

	e.Use(echomiddleware.RequestLoggerWithConfig(echomiddleware.RequestLoggerConfig{
		Skipper: hs.loggingSkipper.LoggingSkipper,
		BeforeNextFunc: func(c echo.Context) {
			if hs.loggingSkipper != nil {
				for _, skipper := range hs.loggingSkipper.BeforeNextSkippers {
					if skipper(c) {
						return
					}
				}
			}

			req := c.Request()
			ctx := req.Context()
			method, path := req.Method, req.URL.Path
			logWiths := make([]any, 0)

			requestCustomObject := map[string]any{
				"headers": transformHeader(req.Header),
			}

			if query := req.URL.Query(); len(query) != 0 {
				requestCustomObject["query"] = query
			}

			if req.Header.Get(echo.HeaderContentType) == echo.MIMEApplicationJSON && req.Body != nil {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					logWiths = append(logWiths, zap.Any("request", requestCustomObject))
					logger.Context(ctx).With(logWiths...).Infof("handling request %s %s", method, path)
					return
				}
				defer req.Body.Close()

				// Restore the request body for further use
				req.Body = io.NopCloser(bytes.NewReader(body))

				var payload any
				if err = json.Unmarshal(body, &payload); err == nil && payload != nil {
					requestCustomObject["payload"] = payload
				}
			}

			logWiths = append(logWiths, zap.Any("request", requestCustomObject))

			logger.Context(ctx).With(logWiths...).Infof("handling %s %s", method, path)
		},
		LogLatency: true,
		LogValuesFunc: func(c echo.Context, v echomiddleware.RequestLoggerValues) error {
			for _, skipper := range hs.loggingSkipper.LogValuesSkippers {
				if skipper(c) {
					return nil
				}
			}

			req := c.Request()
			ctx := req.Context()

			fields := []any{
				zap.String("method", v.Method),
				zap.String("url", v.URI),
				zap.String("status", fmt.Sprintf("%d %s", v.Status, http.StatusText(v.Status))),
				zap.String("latency", v.Latency.String()),
				zap.String("userAgent", v.UserAgent),
				zap.Time("time", v.StartTime),
			}
			if queryParams := req.URL.Query(); len(queryParams) > 0 {
				fields = append(fields, zap.Any("query", queryParams))
			}

			if response := ctx.Value("response"); response != nil {
				fields = append(fields, zap.Any("response", response))
			}

			if v.Error != nil && !errors.Is(v.Error, http.ErrBodyNotAllowed) {
				fields = append(fields, zap.Error(v.Error))
			}

			logger.Context(ctx).With(fields...).Infof("handled %s %s %d %s %s", v.Method, req.URL.Path, v.Status, http.StatusText(v.Status), v.Latency.String())
			return nil
		},
		LogMethod:    true,
		LogURI:       true,
		LogStatus:    true,
		LogError:     true,
		LogUserAgent: true,
	}))

	e.Use(echomiddleware.BodyDump(func(c echo.Context, reqBody, resBody []byte) {
		req, ctx := c.Request(), c.Request().Context()
		responseCustomObject := map[string]any{
			"headers": transformHeader(c.Response().Header()),
		}

		if req.Method != http.MethodGet {
			if resBody != nil && req.Header.Get(echo.HeaderContentType) == echo.MIMEApplicationJSON {
				var data any
				if err := json.Unmarshal(resBody, &data); err == nil && data != nil {
					responseCustomObject["data"] = data
				}
			}
		}

		c.Set("response", responseCustomObject)
		ctx = context.WithValue(ctx, "response", responseCustomObject)
		*req = *req.WithContext(ctx)
	}))

	if len(hs.middlewares) > 0 {
		e.Use(hs.middlewares...)
	}

	hs.router = e

	return hs
}

func (hs *httpServer) ListenAndServe() {
	go func() {
		if err := hs.router.Start(":" + strconv.Itoa(hs.port)); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("httpserver: listen and serve at port %d: %s", hs.port, err.Error())
		}
	}()
}

func (hs *httpServer) GracefulShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	signal.Notify(quit, syscall.SIGINT)
	signal.Notify(quit, os.Interrupt)
	<-quit
	logger.Info("received a sigterm, sigint, or os interrupt signal -> graceful shutting down")
	logger.Info("httpserver: shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := hs.router.Shutdown(ctx); err != nil {
		logger.Fatalf("httpserver: shutdown: %s", err.Error())
	}
	logger.Info("httpserver: shutdown")
}

func (hs *httpServer) Routers() *echo.Echo {
	return hs.router
}

func transformHeader(headers http.Header) map[string]any {
	header := make(map[string]any)
	for key, values := range headers {
		if len(values) == 1 {
			header[key] = headers.Get(key)
		} else {
			header[key] = values
		}
	}
	return header
}

type CustomObjectMarshaler map[string]interface{}

func (c CustomObjectMarshaler) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	for key, value := range c {
		switch typeOfValue := value.(type) {
		case string:
			enc.AddString(key, typeOfValue)
		case uint:
			enc.AddUint(key, typeOfValue)
		case int64:
			enc.AddInt64(key, typeOfValue)
		case float64:
			enc.AddFloat64(key, typeOfValue)
		case http.Header:
			newMap := make(CustomObjectMarshaler)
			for k, v := range typeOfValue {
				newMap[k] = v[0]
			}
			enc.AddObject(key, newMap)

		case map[string]interface{}:
			newMap := make(CustomObjectMarshaler)
			for k, v := range typeOfValue {
				newMap[k] = v
			}
			enc.AddObject(key, newMap)

		case time.Time:
			enc.AddTime(key, typeOfValue)
		}
	}

	return nil
}

func HealthCheckSkipper(c echo.Context) bool {
	req := c.Request()
	return (req.Method == http.MethodGet && req.URL.Path == "/livez") ||
		(req.Method == http.MethodGet && req.URL.Path == "/readyz")
}
