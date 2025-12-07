package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/thanhthanh221/msa-core/pkg/common"
	services "github.com/thanhthanh221/msa-core/pkg/service"
)

// JWTAuthMiddleware handles JWT authentication for API calls
type JWTAuthMiddleware struct {
	logger     *logrus.Logger
	jwtService services.JWTService
}

// NewJWTAuthMiddleware creates a new JWT auth middleware
func NewJWTAuthMiddleware(secretKey string, logger *logrus.Logger) *JWTAuthMiddleware {
	return &JWTAuthMiddleware{
		logger:     logger,
		jwtService: services.NewJWTService(secretKey),
	}
}

// RequireAuth middleware that validates JWT tokens
func (m *JWTAuthMiddleware) RequireAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get Authorization header
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error":             "missing_authorization_header",
					"error_description": "Authorization header is required",
				})
			}

			// Check if it's a Bearer token
			if !strings.HasPrefix(authHeader, "Bearer ") {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error":             "invalid_authorization_header",
					"error_description": "Authorization header must start with 'Bearer '",
				})
			}

			// Extract token
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error":             "missing_token",
					"error_description": "Token is required",
				})
			}

			// Validate token
			claims, err := m.jwtService.ValidateToken(token)
			if err != nil {
				m.logger.Warn("Invalid JWT token: ", err)
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error":             "invalid_token",
					"error_description": "Invalid or expired token",
				})
			}

			// Put principal into both Echo context and request context (typed key)
			c.Set("user", &claims.User)
			c.Set("scopes", claims.Scopes)
			c.Set("claims", claims)
			c.Set("user_id", claims.User.ID)

			req := c.Request()
			goCtx := common.WithUserID(req.Context(), claims.User.ID)
			c.SetRequest(req.WithContext(goCtx))

			return next(c)
		}
	}
}

// RequireScope middleware that checks if user has required scope
func (m *JWTAuthMiddleware) RequireScope(requiredScope string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get scopes from context (set by RequireAuth)
			scopes, ok := c.Get("scopes").([]string)
			if !ok {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error":             "missing_scopes",
					"error_description": "No scopes found in token",
				})
			}

			// Check if required scope is present
			hasScope := slices.Contains(scopes, requiredScope)

			if !hasScope {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error":             "insufficient_scope",
					"error_description": "Insufficient scope. Required: " + requiredScope,
				})
			}

			return next(c)
		}
	}
}
