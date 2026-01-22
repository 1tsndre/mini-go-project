package middleware

import (
	"net/http"

	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
)

// MethodNotAllowed intercepts 405 responses from the ServeMux and converts
// them to JSON, consistent with all other API error responses.
func MethodNotAllowed(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&methodNotAllowedWriter{ResponseWriter: w, r: r}, r)
	})
}

type methodNotAllowedWriter struct {
	http.ResponseWriter
	r           *http.Request
	intercepted bool
}

func (m *methodNotAllowedWriter) WriteHeader(code int) {
	if code == http.StatusMethodNotAllowed {
		m.intercepted = true
		meta := BuildMeta(m.r)
		response.ErrorResponse(m.ResponseWriter, code, meta,
			response.NewError(constant.ErrCodeValidation, "method not allowed"),
		)
		return
	}
	m.ResponseWriter.WriteHeader(code)
}

func (m *methodNotAllowedWriter) Write(b []byte) (int, error) {
	if m.intercepted {
		return len(b), nil
	}
	return m.ResponseWriter.Write(b)
}
