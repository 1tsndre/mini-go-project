package middleware

import (
	"net/http"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/logger"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := newResponseWriter(w)

		next.ServeHTTP(rw, r)

		logger.Info(r.Context(), "request completed", map[string]interface{}{
			"method":  r.Method,
			"path":    r.URL.Path,
			"status":  rw.statusCode,
			"latency": time.Since(start).String(),
			"ip":      r.RemoteAddr,
		})
	})
}
