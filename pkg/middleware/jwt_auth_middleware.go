package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/thanhthanh221/msa-core/pkg/common"
	"github.com/thanhthanh221/msa-core/pkg/infrastructure/redis"
	services "github.com/thanhthanh221/msa-core/pkg/service"
)

// JWTAuthMiddleware handles JWT authentication for API calls
// JWTAuthMiddleware handles JWT authentication for API calls
type JWTAuthMiddleware struct {
	logger     *logrus.Logger
	jwtService services.JWTService
}

// NewJWTAuthMiddleware creates a new JWT auth middleware
func NewJWTAuthMiddleware(secretKey string, redisClient redis.RedisClient, logger *logrus.Logger) *JWTAuthMiddleware {
	return &JWTAuthMiddleware{
		logger:     logger,
		jwtService: services.NewJWTService(secretKey, redisClient),
	}
}

// RequireAuth middleware that validates JWT tokens
func (m *JWTAuthMiddleware) RequireAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error":             "missing_authorization_header",
					"error_description": "Authorization header is required",
				})
			}

			if !strings.HasPrefix(authHeader, "Bearer ") {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error":             "invalid_authorization_header",
					"error_description": "Authorization header must start with 'Bearer '",
				})
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error":             "missing_token",
					"error_description": "Token is required",
				})
			}

			claims, err := m.jwtService.ValidateToken(token)
			if err != nil {
				m.logger.Warn("Invalid JWT token: ", err)
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error":             "invalid_token",
					"error_description": err.Error(),
				})
			}

			InjectAuth(c, &claims.User, claims.Scopes, claims.SID)
			c.Set("claims", claims)

			return next(c)
		}
	}
}

// RequireScope middleware that checks if user has required scope
func (m *JWTAuthMiddleware) RequireScope(requiredScope string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			scopes, ok := common.Scopes(c.Request().Context())
			if !ok || len(scopes) == 0 {
				authHeader := c.Request().Header.Get("Authorization")
				if authHeader == "" {
					return c.JSON(http.StatusUnauthorized, map[string]string{
						"error":             "missing_authorization_header",
						"error_description": "Authorization header is required",
					})
				}

				if !strings.HasPrefix(authHeader, "Bearer ") {
					return c.JSON(http.StatusUnauthorized, map[string]string{
						"error":             "invalid_authorization_header",
						"error_description": "Authorization header must start with 'Bearer '",
					})
				}

				token := strings.TrimPrefix(authHeader, "Bearer ")
				if token == "" {
					return c.JSON(http.StatusUnauthorized, map[string]string{
						"error":             "missing_token",
						"error_description": "Token is required",
					})
				}

				claims, err := m.jwtService.ValidateToken(token)
				if err != nil {
					m.logger.Warn("Invalid JWT token: ", err)
					return c.JSON(http.StatusUnauthorized, map[string]string{
						"error":             "invalid_token",
						"error_description": err.Error(),
					})
				}

				scopes = claims.Scopes
				InjectAuth(c, &claims.User, claims.Scopes, claims.SID)
				c.Set("claims", claims)
			}

			if !hasScope(scopes, requiredScope) {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error":             "insufficient_scope",
					"error_description": "Insufficient scope. Required: " + requiredScope,
				})
			}

			return next(c)
		}
	}
}

// RequireRole middleware that checks if user has required role
func (m *JWTAuthMiddleware) RequireRole(requiredRole string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			roles, ok := common.Roles(c.Request().Context())
			if !ok || len(roles) == 0 {
				if u, ok := UserFromGoContext(c.Request().Context()); ok && u != nil && len(u.Roles) > 0 {
					roles = u.Roles
					Bind(c, func(rc *common.RequestContext) {
						rc.Roles = roles
					})
				}
			}

			if len(roles) == 0 {
				authHeader := c.Request().Header.Get("Authorization")
				if authHeader == "" {
					return c.JSON(http.StatusUnauthorized, map[string]string{
						"error":             "missing_authorization_header",
						"error_description": "Authorization header is required",
					})
				}

				if !strings.HasPrefix(authHeader, "Bearer ") {
					return c.JSON(http.StatusUnauthorized, map[string]string{
						"error":             "invalid_authorization_header",
						"error_description": "Authorization header must start with 'Bearer '",
					})
				}

				token := strings.TrimPrefix(authHeader, "Bearer ")
				if token == "" {
					return c.JSON(http.StatusUnauthorized, map[string]string{
						"error":             "missing_token",
						"error_description": "Token is required",
					})
				}

				claims, err := m.jwtService.ValidateToken(token)
				if err != nil {
					m.logger.Warn("Invalid JWT token: ", err)
					return c.JSON(http.StatusUnauthorized, map[string]string{
						"error":             "invalid_token",
						"error_description": err.Error(),
					})
				}

				roles = claims.User.Roles
				InjectAuth(c, &claims.User, claims.Scopes, claims.SID)
				c.Set("claims", claims)
			}

			if len(roles) == 0 {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error":             "missing_role",
					"error_description": "No role found in token",
				})
			}

			if !slices.Contains(roles, requiredRole) {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error":             "insufficient_role",
					"error_description": "Insufficient role. Required: " + requiredRole,
				})
			}

			return next(c)
		}
	}
}

func hasScope(grantedScopes []string, required string) bool {
	if required == "" {
		return false
	}

	for _, granted := range grantedScopes {
		if granted == "" {
			continue
		}
		if granted == "*" {
			return true
		}
		if granted == required {
			return true
		}
		if strings.HasSuffix(granted, ".*") {
			prefix := strings.TrimSuffix(granted, ".*")
			if required == prefix || strings.HasPrefix(required, prefix+".") {
				return true
			}
		}
	}

	return false
}
