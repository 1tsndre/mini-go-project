package handler

import (
	"errors"

	"github.com/1tsndre/mini-go-project/store-service/internal/service"
)

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
