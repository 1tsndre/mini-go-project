package middleware

import (
	"net/http"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/google/uuid"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(constant.HeaderRequestID)
		// The ID goes into every log line, so only a UUID from the client is kept.
		if _, err := uuid.Parse(requestID); err != nil {
			requestID = uuid.NewString()
		}

		ctx := logger.WithRequestID(r.Context(), requestID)
		w.Header().Set(constant.HeaderRequestID, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
