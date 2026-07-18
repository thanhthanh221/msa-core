package middleware

import (
	"context"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/thanhthanh221/msa-core/pkg/common"
	"github.com/thanhthanh221/msa-core/pkg/models"
)

type userCtxKey struct{}

// Bind merges mutate into RequestContext and writes it back to the request's
// Go context. Echo c.Set is only a thin mirror for handler/middleware that still
// use c.Get — Go context is the source of truth for services.
func Bind(c echo.Context, mutate func(*common.RequestContext)) {
	req := c.Request()
	rc, _ := common.Req(req.Context())
	mutate(&rc)
	c.SetRequest(req.WithContext(common.WithRequest(req.Context(), rc)))
	mirrorEcho(c, rc)
}

func mirrorEcho(c echo.Context, rc common.RequestContext) {
	if rc.UserID != "" {
		c.Set("user_id", rc.UserID)
	}
	if rc.SID != "" {
		c.Set("sid", rc.SID)
	}
	if rc.Locale != "" {
		c.Set("locale", rc.Locale)
	}
	if rc.TraceID != "" {
		c.Set("trace_id", rc.TraceID)
	}
	if len(rc.Scopes) > 0 {
		c.Set("scopes", rc.Scopes)
	}
	if len(rc.Roles) > 0 {
		c.Set("roles", rc.Roles)
	}
}

// InjectAuth populates auth fields on RequestContext from a validated principal.
func InjectAuth(c echo.Context, user *models.OAuthUser, scopes []string, sid string) {
	Bind(c, func(rc *common.RequestContext) {
		if user != nil {
			rc.UserID = user.ID
			if len(user.Roles) > 0 {
				rc.Roles = user.Roles
			}
		}
		if sid != "" {
			rc.SID = sid
		}
		if len(scopes) > 0 {
			rc.Scopes = scopes
		}
	})

	if user != nil {
		req := c.Request()
		c.SetRequest(req.WithContext(context.WithValue(req.Context(), userCtxKey{}, user)))
		c.Set("user", user)
	}
}

// UserFromGoContext returns the authenticated user stored beside RequestContext.
func UserFromGoContext(ctx context.Context) (*models.OAuthUser, bool) {
	user, ok := ctx.Value(userCtxKey{}).(*models.OAuthUser)
	return user, ok && user != nil
}

// LocaleMiddleware is the shared request bootstrap middleware.
// Register after tracing. Seeds RequestContext with locale, trace id, and other
// request-scoped helpers so services can use ctx.Request().Context().
func LocaleMiddleware() echo.MiddlewareFunc {
	return RequestContextMiddleware()
}

// RequestContextMiddleware seeds common RequestContext fields for every request.
func RequestContextMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			locale := common.GetLocaleFromHeader(c.Request().Header)
			traceID := resolveTraceID(c)

			Bind(c, func(rc *common.RequestContext) {
				if rc.Locale == "" {
					rc.Locale = locale
				}
				if rc.TraceID == "" {
					rc.TraceID = traceID
				}
			})

			// Expose correlation id to clients / logs
			if rc, ok := common.Req(c.Request().Context()); ok && rc.TraceID != "" {
				c.Response().Header().Set("X-Trace-Id", rc.TraceID)
			}

			return next(c)
		}
	}
}

func resolveTraceID(c echo.Context) string {
	if c == nil || c.Request() == nil {
		return uuid.NewString()
	}
	if id := c.Request().Header.Get("X-Trace-Id"); id != "" {
		return id
	}
	if id := c.Request().Header.Get("X-Request-Id"); id != "" {
		return id
	}
	return uuid.NewString()
}
