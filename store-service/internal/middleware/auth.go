package middleware

import (
	"context"
	"net/http"
	"strings"

	pkgconstant "github.com/1tsndre/mini-go-project/pkg/constant"
	"github.com/1tsndre/mini-go-project/pkg/jwt"
	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
)

func Auth(jwtManager *jwt.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get(constant.HeaderAuthorization)
			if authHeader == "" {
				response.Failure(w, http.StatusUnauthorized, BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgMissingAuthHeader})
				return
			}

			scheme, token, ok := strings.Cut(authHeader, " ")
			if !ok || scheme != constant.BearerScheme {
				response.Failure(w, http.StatusUnauthorized, BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidAuthFormat})
				return
			}

			claims, err := jwtManager.ValidateToken(token)
			if err != nil {
				response.Failure(w, http.StatusUnauthorized, BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidOrExpiredToken})
				return
			}
			if claims.Type != pkgconstant.TokenTypeAccess {
				response.Failure(w, http.StatusUnauthorized, BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidTokenType})
				return
			}

			ctx := context.WithValue(r.Context(), ContextUserID, claims.UserID)
			ctx = context.WithValue(ctx, ContextRole, claims.Role)
			ctx = logger.WithUserID(ctx, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Context().Value(ContextRole) != role {
				response.Failure(w, http.StatusForbidden, BuildMeta(r), response.Error{Code: constant.ErrCodeForbidden, Message: constant.MsgInsufficientPermissions})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
