package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/1tsndre/mini-go-project/store-service/internal/middleware"
	"github.com/stretchr/testify/assert"
)

func multipartRequest(t *testing.T, field string, size int) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile(field, "photo.png")
	assert.NoError(t, err)
	_, err = part.Write(bytes.Repeat([]byte("a"), size))
	assert.NoError(t, err)
	assert.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestIsBodyTooLarge_FormFile(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		limit    int64
		wantErr  bool
		tooLarge bool
	}{
		{name: "within limit", size: 1 << 10, limit: 1 << 20},
		{name: "over limit", size: 2 << 20, limit: 1 << 20, wantErr: true, tooLarge: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var formErr error
			h := middleware.MaxBodyBytes(tt.limit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _, formErr = r.FormFile("image")
			}))
			h.ServeHTTP(httptest.NewRecorder(), multipartRequest(t, "image", tt.size))

			if !tt.wantErr {
				assert.NoError(t, formErr)
				return
			}
			assert.Error(t, formErr)
			assert.Equal(t, tt.tooLarge, isBodyTooLarge(formErr))
		})
	}
}

func TestIsBodyTooLarge_OtherErrors(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/upload", nil)
	_, _, err := req.FormFile("image")
	assert.Error(t, err)
	assert.False(t, isBodyTooLarge(err))
}
