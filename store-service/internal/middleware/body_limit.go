package middleware

import "net/http"

// MaxBodyBytes caps the size of every request body. Reads past the limit fail,
// so JSON decoding and multipart parsing stop early instead of buffering
// arbitrarily large payloads.
func MaxBodyBytes(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}
