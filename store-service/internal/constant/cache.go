package constant

import "time"

const (
	KeyProduct   = "product:%s"
	KeyCart      = "cart:%s"
	KeyUser      = "user:%s"
	KeyRateLimit = "rate_limit:%s:%s"
	KeyCartLock  = "cart_lock:%s"
	// KeyRefreshToken holds the user ID of a refresh token that may still be used, by jti.
	KeyRefreshToken = "refresh_token:%s"
)

const (
	TTLProduct = 15 * time.Minute
	TTLCart    = 0 // no expiry
)
