package constant

const (
	// MaxVarcharLength is the size of the VARCHAR(255) columns (emails and names).
	// Longer input has to be rejected up front: PostgreSQL would refuse it and the
	// request would end as a 500 instead of a 400.
	MaxVarcharLength = 255
	// MaxPasswordBytes is the longest password bcrypt accepts.
	MaxPasswordBytes = 72
)
