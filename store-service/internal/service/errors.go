package service

// Error is safe to show to the client. Code is one of constant.ErrCode*.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Message
}
