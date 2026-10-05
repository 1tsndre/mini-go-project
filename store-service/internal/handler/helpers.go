package handler

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/service"
)

var (
	maxVarcharMessage  = fmt.Sprintf("maximum %d characters", constant.MaxVarcharLength)
	maxPasswordMessage = fmt.Sprintf("maximum %d bytes", constant.MaxPasswordBytes)
)

// exceedsVarchar reports whether s is longer than a VARCHAR(255) column holds.
// PostgreSQL counts characters, not bytes.
func exceedsVarchar(s string) bool {
	return utf8.RuneCountInString(s) > constant.MaxVarcharLength
}

// isInsufficientStock reports whether err means a product cannot cover the
// requested quantity. Matched by type: the message may contain a product name.
func isInsufficientStock(err error) bool {
	return errors.Is(err, service.ErrInsufficientStock)
}

// isInvalidStatusTransition reports whether err means an order cannot move to the
// requested status. Matched by type: the message contains the requested status.
func isInvalidStatusTransition(err error) bool {
	return errors.Is(err, service.ErrInvalidStatusTransition)
}
