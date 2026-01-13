package constant

import "time"

const (
	KeyProduct      = "product:%s"
	KeyRateLimit    = "rate_limit:%s:%s"
	KeyRefreshToken = "refresh_token:%s"
)

const (
	TTLProduct = 15 * time.Minute
)
