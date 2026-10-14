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
	// KeyIdempotency holds the response to a user's request with an Idempotency-Key.
	KeyIdempotency = "idempotency:%s:%s"
)

const (
	TTLProduct = 15 * time.Minute
	TTLCart    = 0 // no expiry
	// TTLIdempotency is how long a response is replayed for a retried Idempotency-Key.
	TTLIdempotency = 24 * time.Hour
	// TTLIdempotencyPending bounds how long a request that never finished, e.g.
	// because the service stopped, blocks retries with its key.
	TTLIdempotencyPending = time.Minute
)
