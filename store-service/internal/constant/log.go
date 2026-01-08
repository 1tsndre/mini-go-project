package constant

const (
	LogConfigLoadFailed     = "failed to load config: %v"
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
)
