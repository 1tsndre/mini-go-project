package constant

import "time"

const (
	TopicOrderCreated   = "order.created"
	TopicPaymentSuccess = "payment.success"
	TopicPaymentFailed  = "payment.failed"

	ChannelStoreService = "store-service"

	NSQHandleTimeout = 30 * time.Second
	NSQMaxAttempts   = 0 // never give up on a message
)
