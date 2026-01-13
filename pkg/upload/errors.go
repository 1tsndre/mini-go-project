package upload

import "errors"

// ErrInvalidFile is a problem with the uploaded file itself.
var ErrInvalidFile = errors.New("invalid file")
