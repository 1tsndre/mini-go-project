package constant

const (
	MaxVarcharLength  = 255 // VARCHAR(255) columns; PostgreSQL counts characters, not bytes
	MaxPasswordBytes  = 72  // bcrypt limit
	MinPasswordLength = 6

	MaxDescriptionLength = 5000

	PriceDecimals  = 2
	MaxPriceLength = 32

	// PostgreSQL rejects the NUL character.
	EscapedNUL = `\u0000`
)
