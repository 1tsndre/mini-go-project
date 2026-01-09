package constant

const (
	LogConfigLoadFailed     = "failed to load config: %v"
	LogWeakJWTSecret        = "JWT_SECRET must be set to a strong, non-default value"
	LogDBConnectFailed      = "failed to connect to database"
	LogDBConnected          = "connected to database"
	LogRedisConnectFailed   = "failed to connect to redis"
	LogRedisConnected       = "connected to redis"
	LogServerStarting       = "server starting"
	LogServerFailed         = "server failed"
	LogServerShuttingDown   = "shutting down server..."
	LogServerForcedShutdown = "server forced to shutdown"
	LogServerStopped        = "server stopped"

	LogRequestCompleted = "request completed"
	LogPanicRecovered   = "panic recovered"
	LogQueryFailed      = "query failed"
	LogQuery            = "query"

	LogPasswordHashFailed        = "failed to hash password"
	LogUserCreateFailed          = "failed to create user"
	LogUserLoadFailed            = "failed to load user"
	LogTokenPairFailed           = "failed to generate token pair"
	LogRefreshTokenStoreFailed   = "failed to store refresh token"
	LogRefreshTokenConsumeFailed = "failed to consume refresh token"
	LogRefreshTokenRevokeFailed  = "failed to revoke refresh token"
	LogUserRegistered            = "user registered"
	LogUserLoggedIn              = "user logged in"
	LogUserLoggedOut             = "user logged out"
)
