package router

import (
	"net/http"
	"strings"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/middleware"
)

func NewRouter(maxBodyBytes int64, requestTimeout time.Duration) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		meta := middleware.BuildMeta(r)
		response.Success(w, http.StatusOK, map[string]string{"status": "ok"}, meta)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		response.Failure(w, http.StatusNotFound, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeNotFound, Message: constant.MsgNotFound})
	})

	mux.Handle("GET /docs/", http.StripPrefix("/docs/", fileServer("./docs")))

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
