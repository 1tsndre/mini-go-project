package logger

import (
	"context"
	"os"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/constant"
	"github.com/rs/zerolog"
)

type Fields = map[string]interface{}

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	userIDKey    contextKey = "user_id"
)

var log zerolog.Logger

func Init(env string) {
	if env == constant.EnvDevelopment {
		log = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).With().Timestamp().Logger()
	} else {
		log = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func GetRequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey).(string)
	return requestID
}

func GetUserID(ctx context.Context) string {
	userID, _ := ctx.Value(userIDKey).(string)
	return userID
}

func Info(ctx context.Context, msg string, fields ...Fields) {
	write(ctx, log.Info(), msg, fields)
}

func Error(ctx context.Context, msg string, err error, fields ...Fields) {
	write(ctx, log.Error().Err(err), msg, fields)
}

func Warn(ctx context.Context, msg string, fields ...Fields) {
	write(ctx, log.Warn(), msg, fields)
}

func Debug(ctx context.Context, msg string, fields ...Fields) {
	write(ctx, log.Debug(), msg, fields)
}

func Fatal(ctx context.Context, msg string, err error, fields ...Fields) {
	write(ctx, log.Fatal().Err(err), msg, fields)
}

func write(ctx context.Context, event *zerolog.Event, msg string, fields []Fields) {
	event = event.Caller(2) // skip write and its caller
	if requestID := GetRequestID(ctx); requestID != "" {
		event = event.Str("request_id", requestID)
	}
	if userID := GetUserID(ctx); userID != "" {
		event = event.Str("user_id", userID)
	}
	for _, f := range fields {
		event = event.Fields(f)
	}
	event.Msg(msg)
}
