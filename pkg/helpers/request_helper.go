package helpers

import (
	"github.com/labstack/echo/v4"
)

func GetTraceId(c echo.Context) string {
	if c == nil || c.Request() == nil {
		return ""
	}
	return c.Request().Header.Get("X-Trace-Id")
}
