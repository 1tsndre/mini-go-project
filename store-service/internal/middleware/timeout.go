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

// timeoutWriter gives the handler goroutine its own header map. The real
// ResponseWriter is only touched under mu, and never by the handler once the
// request has timed out, so the handler and the timeout response can never
// write the same header map concurrently (a fatal "concurrent map writes").
type timeoutWriter struct {
	w           http.ResponseWriter
	h           http.Header
	mu          sync.Mutex
	wroteHeader bool
	timedOut    bool
}

func (tw *timeoutWriter) Header() http.Header {
	return tw.h
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut || tw.wroteHeader {
		return
	}
	tw.writeHeaderLocked(code)
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return 0, http.ErrHandlerTimeout
	}
	if !tw.wroteHeader {
		tw.writeHeaderLocked(http.StatusOK)
	}
	return tw.w.Write(b)
}

func (tw *timeoutWriter) writeHeaderLocked(code int) {
	dst := tw.w.Header()
	for k, vv := range tw.h {
		dst[k] = vv
	}
	tw.wroteHeader = true
	tw.w.WriteHeader(code)
}

func Timeout(duration time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), duration)
			defer cancel()

			done := make(chan struct{})
			tw := &timeoutWriter{w: w, h: make(http.Header)}

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
					// The response is already on its way; let the handler finish it
					// rather than returning while it still writes to w.
					<-done
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
