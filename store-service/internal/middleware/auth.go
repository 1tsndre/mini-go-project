package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/1tsndre/mini-go-project/pkg/jwt"
	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
)

type contextKey string

const (
	ContextUserID contextKey = "user_id"
	ContextEmail  contextKey = "email"
	ContextRole   contextKey = "role"
)

func Auth(jwtManager *jwt.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get(constant.HeaderAuthorization)
			if authHeader == "" {
				meta := BuildMeta(r)
				response.ErrorResponse(w, http.StatusUnauthorized, meta,
					response.NewError(constant.ErrCodeUnauthorized, "missing authorization header"),
				)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != constant.BearerScheme {
				meta := BuildMeta(r)
				response.ErrorResponse(w, http.StatusUnauthorized, meta,
					response.NewError(constant.ErrCodeUnauthorized, "invalid authorization format"),
				)
				return
			}

			claims, err := jwtManager.ValidateToken(parts[1])
			if err != nil {
				meta := BuildMeta(r)
				response.ErrorResponse(w, http.StatusUnauthorized, meta,
					response.NewError(constant.ErrCodeUnauthorized, "invalid or expired token"),
				)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, ContextUserID, claims.UserID)
			ctx = context.WithValue(ctx, ContextEmail, claims.Email)
			ctx = context.WithValue(ctx, ContextRole, claims.Role)
			ctx = logger.WithUserID(ctx, claims.UserID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := r.Context().Value(ContextRole).(string)
			if !ok {
				meta := BuildMeta(r)
				response.ErrorResponse(w, http.StatusForbidden, meta,
					response.NewError(constant.ErrCodeForbidden, "forbidden"),
				)
				return
			}

			allowed := false
			for _, r := range roles {
				if role == r {
					allowed = true
					break
				}
			}

			if !allowed {
				meta := BuildMeta(r)
				response.ErrorResponse(w, http.StatusForbidden, meta,
					response.NewError(constant.ErrCodeForbidden, "insufficient permissions"),
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func GetUserID(ctx context.Context) string {
	if val, ok := ctx.Value(ContextUserID).(string); ok {
		return val
	}
	return ""
}

func GetUserRole(ctx context.Context) string {
	if val, ok := ctx.Value(ContextRole).(string); ok {
		return val
	}
	return ""
}
