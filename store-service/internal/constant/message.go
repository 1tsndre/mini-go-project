package constant

const (
	MsgInternalServerError = "internal server error"
	MsgNotFound            = "not found"
	MsgRateLimitExceeded   = "rate limit exceeded"

	MsgMissingAuthHeader       = "missing authorization header"
	MsgInvalidAuthFormat       = "invalid authorization format"
	MsgInvalidOrExpiredToken   = "invalid or expired token"
	MsgInvalidTokenType        = "invalid token type"
	MsgInsufficientPermissions = "insufficient permissions"
	MsgAuthUnavailable         = "service temporarily unavailable, please try again later"
)
