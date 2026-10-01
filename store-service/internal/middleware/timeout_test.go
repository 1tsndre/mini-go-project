package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/stretchr/testify/assert"
)

func TestTimeout_PassesThroughFastResponse(t *testing.T) {
	h := Timeout(time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Handler", "yes")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "yes", rec.Header().Get("X-Handler"))
	assert.Equal(t, "created", rec.Body.String())
}

func TestTimeout_ImplicitStatusOK(t *testing.T) {
	h := Timeout(time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	assert.Equal(t, "ok", rec.Body.String())
}

func TestTimeout_SlowHandlerGets504(t *testing.T) {
	// The handler only tries to respond after the middleware has returned the 504,
	// so the outcome does not depend on goroutine scheduling.
	release := make(chan struct{})
	handlerDone := make(chan struct{})
	h := Timeout(10 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(handlerDone)
		<-release
		w.Header().Set("X-Late", "should-not-leak")
		w.WriteHeader(http.StatusOK)
		n, err := w.Write([]byte("late body"))
		assert.Zero(t, n)
		assert.ErrorIs(t, err, http.ErrHandlerTimeout)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	close(release)
	<-handlerDone

	assert.Equal(t, http.StatusGatewayTimeout, rec.Code)
	assert.Empty(t, rec.Header().Get("X-Late"))

	var body response.Response
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	if assert.Len(t, body.Errors, 1) {
		assert.Equal(t, constant.ErrCodeTimeout, body.Errors[0].Code)
	}
}

func TestTimeout_StartedResponseIsCompletedNotReplaced(t *testing.T) {
	// The generous deadline only has to outlast the handler goroutine starting and
	// writing its header, even on a busy CI machine.
	h := Timeout(300 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		<-r.Context().Done()
		time.Sleep(10 * time.Millisecond)
		w.Write([]byte("finished"))
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	// ServeHTTP must not return before the handler finished writing.
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "finished", rec.Body.String())
}

// Regression test: a handler whose DB/gRPC call returns as soon as the request
// context is cancelled writes its error response at the same moment the
// middleware writes the 504. Sharing one header map used to crash the whole
// process with "fatal error: concurrent map writes".
func TestTimeout_NoConcurrentHeaderWrites(t *testing.T) {
	h := Timeout(time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		for i := 0; i < 50; i++ {
			w.Header().Set(fmt.Sprintf("X-Debug-%d", i), "v")
		}
		response.ErrorResponse(w, http.StatusInternalServerError, BuildMeta(r),
			response.NewError(constant.ErrCodeInternal, "db error"))
	}))

	// Whichever side writes first wins; the response must be exactly one of the
	// two, never a crash or a mix of both.
	wantCode := map[int]string{
		http.StatusGatewayTimeout:      constant.ErrCodeTimeout,
		http.StatusInternalServerError: constant.ErrCodeInternal,
	}

	var wg sync.WaitGroup
	for i := 0; i < 5000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

			code, ok := wantCode[rec.Code]
			if !assert.True(t, ok, "unexpected status %d", rec.Code) {
				return
			}
			var body response.Response
			if assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body)) && assert.Len(t, body.Errors, 1) {
				assert.Equal(t, code, body.Errors[0].Code)
			}
		}()
		if i%500 == 0 {
			wg.Wait()
		}
	}
	wg.Wait()
}
