package constant

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
