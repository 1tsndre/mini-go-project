package middleware

import (
	"net/http"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
)

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error(r.Context(), "panic recovered", nil, map[string]interface{}{
					"error": err,
				})

				meta := BuildMeta(r)
				response.ErrorResponse(w, http.StatusInternalServerError, meta,
					response.NewError(constant.ErrCodeInternal, "internal server error"),
				)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
