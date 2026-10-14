package constant

import "time"

const (
	// OutboxRelayInterval is how often the outbox relay publishes queued messages.
	OutboxRelayInterval = time.Second
	// OutboxRelayBatchSize caps how many messages the relay publishes per run.
	OutboxRelayBatchSize = 100
)

const (
	TopicOrderCreated   = "order.created"
	TopicPaymentSuccess = "payment.success"
	TopicPaymentFailed  = "payment.failed"

	ChannelPaymentService = "payment-service"
	ChannelStoreService   = "store-service"
)
