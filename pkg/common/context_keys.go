package common

import "context"

// Compatibility helpers — prefer Req(ctx) for new code.
// These read/write fields on RequestContext so existing call sites keep working.

func WithUserID(ctx context.Context, userID string) context.Context {
	return MergeRequest(ctx, func(rc *RequestContext) {
		rc.UserID = userID
	})
}

func UserID(ctx context.Context) (string, bool) {
	rc, ok := Req(ctx)
	if !ok || rc.UserID == "" {
		return "", false
	}
	return rc.UserID, true
}

func WithSID(ctx context.Context, sid string) context.Context {
	return MergeRequest(ctx, func(rc *RequestContext) {
		rc.SID = sid
	})
}

func SID(ctx context.Context) (string, bool) {
	rc, ok := Req(ctx)
	if !ok || rc.SID == "" {
		return "", false
	}
	return rc.SID, true
}

func WithScopes(ctx context.Context, scopes []string) context.Context {
	return MergeRequest(ctx, func(rc *RequestContext) {
		rc.Scopes = scopes
	})
}

func Scopes(ctx context.Context) ([]string, bool) {
	rc, ok := Req(ctx)
	if !ok || len(rc.Scopes) == 0 {
		return nil, false
	}
	return rc.Scopes, true
}

func WithRoles(ctx context.Context, roles []string) context.Context {
	return MergeRequest(ctx, func(rc *RequestContext) {
		rc.Roles = roles
	})
}

func Roles(ctx context.Context) ([]string, bool) {
	rc, ok := Req(ctx)
	if !ok || len(rc.Roles) == 0 {
		return nil, false
	}
	return rc.Roles, true
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return MergeRequest(ctx, func(rc *RequestContext) {
		rc.TraceID = traceID
	})
}

func TraceID(ctx context.Context) (string, bool) {
	rc, ok := Req(ctx)
	if !ok || rc.TraceID == "" {
		return "", false
	}
	return rc.TraceID, true
}
