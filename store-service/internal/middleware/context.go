package middleware

import "context"

type contextKey string

const (
	ContextUserID contextKey = "user_id"
	ContextRole   contextKey = "role"
)

func GetUserID(ctx context.Context) string {
	userID, _ := ctx.Value(ContextUserID).(string)
	return userID
}
