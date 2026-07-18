package common

import (
	"context"
	"errors"
)

type requestCtxKey struct{}

// RequestContext holds request-scoped metadata for service/business layers.
// Extend this struct when adding new request metadata (e.g. TenantID).
// Full user object is stored by middleware (avoids common↔models import cycle).
type RequestContext struct {
	UserID  string
	SID     string
	Locale  string
	TraceID string
	Scopes  []string
	Roles   []string
}

// WithRequest stores RequestContext on ctx (single key).
func WithRequest(ctx context.Context, rc RequestContext) context.Context {
	return context.WithValue(ctx, requestCtxKey{}, rc)
}

// Req returns RequestContext if present.
func Req(ctx context.Context) (RequestContext, bool) {
	if ctx == nil {
		return RequestContext{}, false
	}
	rc, ok := ctx.Value(requestCtxKey{}).(RequestContext)
	return rc, ok
}

// MustUserID returns UserID or an error when missing.
func MustUserID(ctx context.Context) (string, error) {
	id, ok := UserID(ctx)
	if !ok || id == "" {
		return "", errors.New("user id not found in context")
	}
	return id, nil
}

// MergeRequest applies mutate onto existing RequestContext (or empty) and stores it.
func MergeRequest(ctx context.Context, mutate func(*RequestContext)) context.Context {
	rc, _ := Req(ctx)
	mutate(&rc)
	return WithRequest(ctx, rc)
}
