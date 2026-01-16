package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
)

type timeoutWriter struct {
	http.ResponseWriter
	mu          sync.Mutex
	wroteHeader bool
	timedOut    bool
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if !tw.timedOut && !tw.wroteHeader {
		tw.wroteHeader = true
		tw.ResponseWriter.WriteHeader(code)
	}
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return 0, nil
	}
	tw.wroteHeader = true
	return tw.ResponseWriter.Write(b)
}

func Timeout(duration time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), duration)
			defer cancel()

			done := make(chan struct{})
			tw := &timeoutWriter{ResponseWriter: w}

			go func() {
				next.ServeHTTP(tw, r.WithContext(ctx))
				close(done)
			}()

			select {
			case <-done:
			case <-ctx.Done():
				tw.mu.Lock()
				if tw.wroteHeader {
					tw.mu.Unlock()
					return
				}
				tw.timedOut = true
				tw.mu.Unlock()

				logger.Error(r.Context(), "request timeout", nil, map[string]any{
					"duration": duration.String(),
					"path":     r.URL.Path,
				})
				meta := BuildMeta(r)
				response.ErrorResponse(w, http.StatusGatewayTimeout, meta,
					response.NewError(constant.ErrCodeTimeout, "request timed out"),
				)
			}
		})
	}
}