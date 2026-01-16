package logger

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/constant"
	"github.com/rs/zerolog"
)

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	userIDKey    contextKey = "user_id"
)

var log zerolog.Logger

func Init(env string) {
	if env == constant.EnvDevelopment {
		output := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			FormatMessage: func(i interface{}) string {
				return fmt.Sprintf("%s", i)
			},
		}
		log = zerolog.New(output).With().Timestamp().Logger()
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
	if val, ok := ctx.Value(requestIDKey).(string); ok {
		return val
	}
	return ""
}

func GetUserID(ctx context.Context) string {
	if val, ok := ctx.Value(userIDKey).(string); ok {
		return val
	}
	return ""
}

func buildEvent(ctx context.Context, event *zerolog.Event) *zerolog.Event {
	layer, pkg, file, funcName := getCallerInfo(3)

	event = event.
		Str("layer", layer).
		Str("package", pkg).
		Str("file", file).
		Str("function", funcName)

	if requestID := GetRequestID(ctx); requestID != "" {
		event = event.Str("request_id", requestID)
	}
	if userID := GetUserID(ctx); userID != "" {
		event = event.Str("user_id", userID)
	}

	return event
}

func Info(ctx context.Context, msg string, fields ...map[string]interface{}) {
	event := buildEvent(ctx, log.Info())
	applyFields(event, fields...)
	event.Msg(msg)
}

func Error(ctx context.Context, msg string, err error, fields ...map[string]interface{}) {
	event := buildEvent(ctx, log.Error())
	if err != nil {
		event = event.Err(err)
	}
	applyFields(event, fields...)
	event.Msg(msg)
}

func Warn(ctx context.Context, msg string, fields ...map[string]interface{}) {
	event := buildEvent(ctx, log.Warn())
	applyFields(event, fields...)
	event.Msg(msg)
}

func Debug(ctx context.Context, msg string, fields ...map[string]interface{}) {
	event := buildEvent(ctx, log.Debug())
	applyFields(event, fields...)
	event.Msg(msg)
}

func Fatal(ctx context.Context, msg string, err error, fields ...map[string]interface{}) {
	event := buildEvent(ctx, log.Fatal())
	if err != nil {
		event = event.Err(err)
	}
	applyFields(event, fields...)
	event.Msg(msg)
}

func applyFields(event *zerolog.Event, fields ...map[string]interface{}) {
	for _, f := range fields {
		for k, v := range f {
			event = event.Interface(k, v)
		}
	}
}

func getCallerInfo(skip int) (layer, pkg, file, funcName string) {
	pc, filePath, _, ok := runtime.Caller(skip)
	if !ok {
		return "unknown", "unknown", "unknown", "unknown"
	}

	parts := strings.Split(filePath, "/")
	file = parts[len(parts)-1]

	// e.g. .../internal/service/user_service.go -> layer=internal, pkg=service
	layer = "unknown"
	pkg = "unknown"
	for i, part := range parts {
		if part == "internal" || part == "pkg" {
			layer = part
			if i+1 < len(parts)-1 {
				pkg = parts[i+1]
			}
			break
		}
	}

	fullFunc := runtime.FuncForPC(pc).Name()

	funcParts := strings.Split(fullFunc, ".")
	funcName = funcParts[len(funcParts)-1]

	return layer, pkg, file, funcName
}
