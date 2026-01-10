package constant

const (
	MsgInternalServerError = "internal server error"
	MsgRequestTimedOut     = "request timed out"
	MsgNotFound            = "not found"
	MsgRateLimitExceeded   = "rate limit exceeded"
	MsgInvalidUser         = "invalid user"
	MsgInvalidRequestBody  = "invalid request body"
	MsgInvalidID           = "invalid %s id"

	MsgIsRequired    = "is required"
	MsgInvalidEmail  = "invalid email format"
	MsgMinCharacters = "minimum %d characters"
	MsgMaxCharacters = "maximum %d characters"
	MsgMaxBytes      = "maximum %d bytes"

	MsgMissingAuthHeader       = "missing authorization header"
	MsgInvalidAuthFormat       = "invalid authorization format"
	MsgInvalidOrExpiredToken   = "invalid or expired token"
	MsgInvalidTokenType        = "invalid token type"
	MsgInvalidRefreshToken     = "invalid refresh token"
	MsgInsufficientPermissions = "insufficient permissions"
	MsgInvalidCredentials      = "invalid email or password"
	MsgAuthUnavailable         = "service temporarily unavailable, please try again later"
	MsgEmailRegistered         = "email already registered"
	MsgUserCreateFailed        = "failed to create user"
	MsgLoggedOut               = "logged out"

	MsgStoreNotFound     = "store not found"
	MsgStoreExists       = "user already has a store"
	MsgNotStoreOwner     = "forbidden: not store owner"
	MsgStoreCreateFailed = "failed to create store"
	MsgStoreUpdateFailed = "failed to update store"
)
