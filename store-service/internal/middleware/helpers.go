package middleware

import (
	"net/http"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/pkg/response"
)

func BuildMeta(r *http.Request) *response.Meta {
	return &response.Meta{
		RequestID: logger.GetRequestID(r.Context()),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

func Chain(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}
