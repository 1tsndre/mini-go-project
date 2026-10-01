package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad_DefaultTimeoutsAreConsistent(t *testing.T) {
	// Empty values fall back to the defaults (viper ignores empty env vars).
	t.Setenv("APP_WRITE_TIMEOUT", "")
	t.Setenv("APP_REQUEST_TIMEOUT", "")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.Equal(t, 30*time.Second, cfg.App.RequestTimeout)
	assert.Greater(t, cfg.App.WriteTimeout, cfg.App.RequestTimeout)
}

func TestLoad_RejectsWriteTimeoutNotAboveRequestTimeout(t *testing.T) {
	tests := []struct {
		name    string
		write   string
		request string
		wantErr bool
	}{
		{name: "write shorter than request", write: "15s", request: "30s", wantErr: true},
		{name: "write equal to request", write: "30s", request: "30s", wantErr: true},
		{name: "write longer than request", write: "35s", request: "30s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APP_WRITE_TIMEOUT", tt.write)
			t.Setenv("APP_REQUEST_TIMEOUT", tt.request)

			_, err := Load()

			if tt.wantErr {
				assert.ErrorContains(t, err, "APP_WRITE_TIMEOUT")
				return
			}
			assert.NoError(t, err)
		})
	}
}
