package router

import (
	"net/http"
	"strings"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/jwt"
	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/config"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/handler"
	"github.com/1tsndre/mini-go-project/store-service/internal/middleware"
	"github.com/redis/go-redis/v9"
)

type middlewareFunc = func(http.Handler) http.Handler

type Handlers struct {
	Auth  *handler.AuthHandler
	Store *handler.StoreHandler
}

func NewRouter(
	handlers Handlers,
	jwtManager *jwt.JWTManager,
	redisClient *redis.Client,
	maxBodyBytes int64,
	requestTimeout time.Duration,
	rateCfg config.RateConfig,
) http.Handler {
	var (
		mux         = http.NewServeMux()
		rateLimiter = middleware.NewRateLimiter(redisClient)
		loginRate   = rateLimiter.Limit(rateCfg.Login, constant.RateLimitKeyLogin)
		publicRate  = rateLimiter.Limit(rateCfg.Public, constant.RateLimitKeyPublic)
		authRate    = rateLimiter.Limit(rateCfg.Auth, constant.RateLimitKeyAuth)
		auth        = middleware.Auth(jwtManager)

		public   = []middlewareFunc{publicRate}
		login    = []middlewareFunc{loginRate}
		session  = []middlewareFunc{authRate}
		loggedIn = []middlewareFunc{auth, authRate}
		buyer    = []middlewareFunc{auth, middleware.RequireRole(constant.RoleBuyer), authRate}
		seller   = []middlewareFunc{auth, middleware.RequireRole(constant.RoleSeller), authRate}
	)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		meta := middleware.BuildMeta(r)
		response.Success(w, http.StatusOK, map[string]string{"status": "ok"}, meta)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		response.Failure(w, http.StatusNotFound, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeNotFound, Message: constant.MsgNotFound})
	})

	mux.Handle("GET /docs/", http.StripPrefix("/docs/", fileServer("./docs")))

	mux.Handle("POST /api/v1/auth/register", middleware.Chain(http.HandlerFunc(handlers.Auth.Register), login...))
	mux.Handle("POST /api/v1/auth/login", middleware.Chain(http.HandlerFunc(handlers.Auth.Login), login...))
	mux.Handle("POST /api/v1/auth/refresh", middleware.Chain(http.HandlerFunc(handlers.Auth.Refresh), session...))
	mux.Handle("POST /api/v1/auth/logout", middleware.Chain(http.HandlerFunc(handlers.Auth.Logout), session...))

	mux.Handle("POST /api/v1/stores", middleware.Chain(http.HandlerFunc(handlers.Store.CreateStore), buyer...))
	mux.Handle("GET /api/v1/stores/me", middleware.Chain(http.HandlerFunc(handlers.Store.GetMyStore), loggedIn...))
	mux.Handle("GET /api/v1/stores/{id}", middleware.Chain(http.HandlerFunc(handlers.Store.GetStore), public...))
	mux.Handle("PUT /api/v1/stores/{id}", middleware.Chain(http.HandlerFunc(handlers.Store.UpdateStore), seller...))

	return middleware.Chain(mux,
		middleware.RequestID,
		middleware.Logging,
		middleware.MaxBodyBytes(maxBodyBytes),
		middleware.Timeout(requestTimeout),
		middleware.Recovery,
	)
}

// fileServer serves files without directory listings.
func fileServer(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			response.Failure(w, http.StatusNotFound, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeNotFound, Message: constant.MsgNotFound})
			return
		}
		files.ServeHTTP(w, r)
	})
}
