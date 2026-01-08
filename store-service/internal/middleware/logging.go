package middleware

import (
	"net/http"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			start = time.Now()
			rw    = &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		)

		next.ServeHTTP(rw, r)

		logger.Info(r.Context(), constant.LogRequestCompleted, logger.Fields{
			"method":  r.Method,
			"path":    r.URL.Path,
			"status":  rw.statusCode,
			"latency": time.Since(start).String(),
			"ip":      r.RemoteAddr,
		})
	})
}
