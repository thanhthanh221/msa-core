package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func APIKeyAuthMiddleware(expectedApiKey string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			apiKey := c.Request().Header.Get("X-Api-Key")

			if apiKey == "" || apiKey != expectedApiKey {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"message": "Invalid or missing API key",
				})
			}
			return next(c)
		}
	}
}
