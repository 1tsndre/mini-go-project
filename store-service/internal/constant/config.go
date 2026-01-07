package constant

const (
	EnvFile = ".env"

	DefaultAppPort            = "8080"
	DefaultAppReadTimeout     = "15s"
	DefaultAppWriteTimeout    = "35s"
	DefaultAppIdleTimeout     = "60s"
	DefaultAppShutdownTimeout = "30s"
	DefaultAppRequestTimeout  = "30s"

	DefaultDBHost     = "localhost"
	DefaultDBPort     = "5432"
	DefaultDBUser     = "postgres"
	DefaultDBPassword = ""
	DefaultDBName     = "mini_go_ecommerce"
	DefaultDBSSLMode  = "disable"

	DefaultRedisHost     = "localhost"
	DefaultRedisPort     = "6379"
	DefaultRedisPassword = ""
	DefaultRedisDB       = 0

	DefaultNsqdAddr = "localhost:4150"

	DefaultJWTAccessExpiry  = "15m"
	DefaultJWTRefreshExpiry = "168h"

	DefaultRateLimitPublic = 60
	DefaultRateLimitAuth   = 120
	DefaultRateLimitLogin  = 10

	DefaultUploadMaxSize = 5 << 20 // 5 MiB
	DefaultUploadDir     = "./uploads"

	DefaultPaymentGRPCAddr = "localhost:50051"

	MsgInvalidDuration      = "invalid %s: %w"
	MsgWriteTimeoutTooShort = "APP_WRITE_TIMEOUT (%s) must be greater than APP_REQUEST_TIMEOUT (%s)"
)
