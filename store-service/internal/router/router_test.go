package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/1tsndre/mini-go-project/store-service/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestRouter_UploadsServeFilesButNotDirectoryListings(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, os.MkdirAll(filepath.Join(dir, "products"), 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "products", "photo.png"), []byte("png-bytes"), 0o644))

	// The upload routes need none of the handlers, JWT or Redis.
	h := NewRouter(Handlers{}, nil, nil, dir, 1<<20, 5*time.Second, config.RateConfig{})

	tests := []struct {
		path     string
		wantCode int
		wantBody string
	}{
		{path: "/uploads/products/photo.png", wantCode: http.StatusOK, wantBody: "png-bytes"},
		{path: "/uploads/", wantCode: http.StatusNotFound},
		{path: "/uploads/products/", wantCode: http.StatusNotFound},
		// FileServer redirects a directory to its trailing-slash form, which is refused above.
		{path: "/uploads/products", wantCode: http.StatusMovedPermanently},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			assert.Equal(t, tt.wantCode, rec.Code)
			if tt.wantBody != "" {
				assert.Equal(t, tt.wantBody, rec.Body.String())
				return
			}
			assert.NotContains(t, rec.Body.String(), "photo.png", "directory contents must not be listed")
		})
	}
}
