package constant

import "time"

const (
	KeyProduct   = "product:%s"
	KeyCart      = "cart:%s"
	KeyUser      = "user:%s"
	KeyRateLimit = "rate_limit:%s:%s"
	KeyStockLock = "stock_lock:%s"
	KeyCartLock  = "cart_lock:%s"
)

const (
	TTLProduct = 15 * time.Minute
	TTLCart    = 0 // no expiry
)
