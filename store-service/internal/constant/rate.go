package constant

import "time"

const (
	RateLimitWindow = time.Minute

	RateLimitKeyPublic = "public"
	RateLimitKeyAuth   = "auth"
	RateLimitKeyLogin  = "login"
)
