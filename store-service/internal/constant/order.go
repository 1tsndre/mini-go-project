package constant

import "time"

const (
	OrderStatusPending    = "pending"
	OrderStatusPaid       = "paid"
	OrderStatusProcessing = "processing"
	OrderStatusShipping   = "shipping"
	OrderStatusShipped    = "shipped"
	OrderStatusCompleted  = "completed"
	OrderStatusCancelled  = "cancelled"
)

var CancellableStatuses = map[string]bool{
	OrderStatusPending:    true,
	OrderStatusPaid:       true,
	OrderStatusProcessing: true,
}

var OrderStatusTransitions = map[string][]string{
	OrderStatusPaid:       {OrderStatusProcessing},
	OrderStatusProcessing: {OrderStatusShipping},
	OrderStatusShipping:   {OrderStatusShipped},
	OrderStatusShipped:    {OrderStatusCompleted},
}

const (
	// PaymentRetryInterval is how often pending orders are checked for a missing payment result.
	PaymentRetryInterval = time.Minute
	// PaymentRetryAfter is how long an order may stay pending before order.created is republished.
	PaymentRetryAfter = 2 * time.Minute
	// PaymentRetryBatchSize caps how many orders are republished per check.
	PaymentRetryBatchSize = 100
)
