package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/1tsndre/mini-go-project/pkg/upload"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/middleware"
	"github.com/stretchr/testify/assert"
)

func multipartRequest(t *testing.T, field string, size int) *http.Request {
	t.Helper()
	var (
		body bytes.Buffer
		mw   = multipart.NewWriter(&body)
	)

	part, err := mw.CreateFormFile(field, "photo.png")
	assert.NoError(t, err)
	_, err = part.Write(bytes.Repeat([]byte("a"), size))
	assert.NoError(t, err)
	assert.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestSaveUpload(t *testing.T) {
	const limit = 1 << 20

	tests := []struct {
		name     string
		req      *http.Request
		wantCode int
		wantOK   bool
	}{
		{name: "saved", req: multipartRequest(t, "image", 1<<10), wantCode: http.StatusOK, wantOK: true},
		{name: "over the body limit", req: multipartRequest(t, "image", 2<<20), wantCode: http.StatusRequestEntityTooLarge},
		{name: "no file", req: httptest.NewRequest(http.MethodPost, "/upload", nil), wantCode: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				dir      = t.TempDir()
				uploader = upload.NewUploader(dir, limit)
				rec      = httptest.NewRecorder()
				path     string
				ok       bool
				h        = middleware.MaxBodyBytes(limit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					path, ok = saveUpload(w, r, uploader, "image", constant.UploadDirProducts)
				}))
			)

			h.ServeHTTP(rec, tt.req)

			assert.Equal(t, tt.wantCode, rec.Code)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.FileExists(t, filepath.Join(dir, path))
			}
		})
	}
}
