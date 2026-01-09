package constant

const (
	MaxVarcharLength  = 255 // VARCHAR(255) columns; PostgreSQL counts characters, not bytes
	MaxPasswordBytes  = 72  // bcrypt limit
	MinPasswordLength = 6

	// PostgreSQL rejects the NUL character.
	EscapedNUL = `\u0000`
)
