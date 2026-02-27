package constant

const (
	TopicOrderCreated   = "order.created"
	TopicPaymentSuccess = "payment.success"
	TopicPaymentFailed  = "payment.failed"

	ChannelPaymentService = "payment-service"

	NSQMaxAttempts = 0 // never give up on a message
)
