package constant

import "time"

const (
	KeyProduct      = "product:%s"
	KeyCart         = "cart:%s"
	KeyRateLimit    = "rate_limit:%s:%s"
	KeyCartLock     = "cart_lock:%s"
	KeyRefreshToken = "refresh_token:%s"
)

const (
	TTLProduct = 15 * time.Minute
	TTLCart    = 0 // no expiry

	// Longer than a request may run, so the lock cannot expire under it.
	CartLockExpiry = time.Minute
)
