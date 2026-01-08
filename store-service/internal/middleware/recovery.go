package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
)

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// net/http panics with this on purpose to abort a response.
				if err == http.ErrAbortHandler {
					panic(err)
				}
				logger.Error(r.Context(), constant.LogPanicRecovered, nil, logger.Fields{
					"error": err,
					"stack": string(debug.Stack()),
				})
				response.Failure(w, http.StatusInternalServerError, BuildMeta(r), response.Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError})
			}
		}()

		next.ServeHTTP(w, r)
	})
}
