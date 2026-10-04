package service

import "errors"

// These errors are wrapped into messages that can contain user-controlled text
// (a product name, a requested status). Handlers must match them with errors.Is
// rather than by searching the message, which that text could otherwise steer
// (e.g. a status of "failed" being reported as a 500).
var (
	ErrInsufficientStock       = errors.New("insufficient stock")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
)
