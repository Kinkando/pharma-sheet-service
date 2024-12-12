package logger

import (
	"context"
	"errors"
	"log"
	"os"
	"syscall"

	"github.com/spf13/viper"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	structuredLogger *zap.Logger
	sugaredLogger    *zap.SugaredLogger
)

func init() {
	New(getEnv())
}

func New(env string) {
	zapCfg := zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.DebugLevel),
		Development: env != "prod",
		Encoding:    "json",
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:     "message",
			LevelKey:       "level",
			TimeKey:        "time",
			NameKey:        "logger",
			CallerKey:      "file",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.MillisDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
			EncodeName:     zapcore.FullNameEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	if env == "local" {
		zapCfg.Encoding = "console"
		zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	l, err := zapCfg.Build(zap.WithCaller(true), zap.AddCallerSkip(1))
	if err != nil {
		log.Fatal(err)
	}
	structuredLogger, sugaredLogger = l, l.Sugar()
}

func Context(ctx context.Context) *zap.SugaredLogger {
	args := make([]any, 0)

	return sugaredLogger.With(args...).WithOptions(zap.AddCallerSkip(-1))
}

func Structure() *zap.Logger {
	return structuredLogger
}

func Info(args ...any) {
	if getEnv() == "local" {
		sugaredLogger.WithOptions(zap.WithCaller(false), zap.AddCallerSkip(-1)).Info(args...)
		return
	}
	sugaredLogger.Info(args...)
}

func Infof(template string, args ...any) {
	if getEnv() == "local" {
		sugaredLogger.WithOptions(zap.WithCaller(false), zap.AddCallerSkip(-1)).Infof(template, args...)
		return
	}
	sugaredLogger.Infof(template, args...)
}

func Infoln(args ...any) {
	if getEnv() == "local" {
		sugaredLogger.WithOptions(zap.WithCaller(false), zap.AddCallerSkip(-1)).Infoln(args...)
		return
	}
	sugaredLogger.Infoln(args...)
}

func Debug(args ...any) {
	sugaredLogger.Debug(args...)
}

func Debugf(template string, args ...any) {
	sugaredLogger.Debugf(template, args...)
}

func Debugln(args ...any) {
	sugaredLogger.Debugln(args...)
}

func Warn(args ...any) {
	if getEnv() == "local" {
		sugaredLogger.WithOptions(zap.WithCaller(false), zap.AddCallerSkip(-1)).Warn(args...)
		return
	}
	sugaredLogger.Warn(args...)
}

func Warnf(template string, args ...any) {
	if getEnv() == "local" {
		sugaredLogger.WithOptions(zap.WithCaller(false), zap.AddCallerSkip(-1)).Warnf(template, args...)
		return
	}
	sugaredLogger.Warnf(template, args...)
}

func Warnln(args ...any) {
	if getEnv() == "local" {
		sugaredLogger.WithOptions(zap.WithCaller(false), zap.AddCallerSkip(-1)).Warnln(args...)
		return
	}
	sugaredLogger.Warnln(args...)
}

func Error(args ...any) {
	sugaredLogger.Error(args...)
}

func Errorf(template string, args ...any) {
	sugaredLogger.Errorf(template, args...)
}

func Errorln(args ...any) {
	sugaredLogger.Errorln(args...)
}

func Fatal(args ...any) {
	sugaredLogger.Fatal(args...)
}

func Fatalf(template string, args ...any) {
	sugaredLogger.Fatalf(template, args...)
}

func Fatalln(args ...any) {
	sugaredLogger.Fatalln(args...)
}

func Panic(args ...any) {
	sugaredLogger.Panic(args...)
}

func Panicf(template string, args ...any) {
	sugaredLogger.Panicf(template, args...)
}

func Panicln(args ...any) {
	sugaredLogger.Panicln(args...)
}

func getEnv() string {
	if env := os.Getenv("string"); len(env) > 0 {
		return env
	} else if env = viper.GetString("env"); len(env) > 0 {
		return env
	}
	return "prod"
}

func Sync() {
	if err := sugaredLogger.Sync(); err != nil && !errors.Is(err, syscall.ENOTTY) {
		Error(err)
	}
}
