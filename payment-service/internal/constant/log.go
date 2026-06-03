package constant

const (
	LogNSQProducerFailed   = "failed to create NSQ producer"
	LogNSQConsumerFailed   = "failed to start NSQ consumer"
	LogGRPCListenFailed    = "failed to listen on gRPC port"
	LogGRPCServerStarting  = "payment gRPC server starting"
	LogGRPCServerFailed    = "gRPC server failed"
	LogServiceShuttingDown = "shutting down payment service..."
	LogServiceStopped      = "payment service stopped"

	LogConsumerStarted      = "NSQ order consumer started"
	LogConsumerStopped      = "NSQ order consumer stopped"
	LogOrderUnmarshalFailed = "failed to unmarshal order.created, skipping"
	LogPaymentProcessed     = "payment processed"
)
