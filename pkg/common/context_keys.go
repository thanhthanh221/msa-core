package common

import "context"

type ctxKey int

const (
	userIDKey ctxKey = iota
	sessionIDKey
)

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func UserID(ctx context.Context) (string, bool) {
	v := ctx.Value(userIDKey)
	id, ok := v.(string)
	return id, ok
}

func WithSID(ctx context.Context, sid string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sid)
}

func SID(ctx context.Context) (string, bool) {
	v := ctx.Value(sessionIDKey)
	id, ok := v.(string)
	return id, ok
}
