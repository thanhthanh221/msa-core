package helpers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/thanhthanh221/msa-core/pkg/common"
)

// ResponseHelper provides helper functions for handling responses in controllers
type ResponseHelper struct{}

// NewResponseHelper creates a new ResponseHelper instance
func NewResponseHelper() *ResponseHelper {
	return &ResponseHelper{}
}

// Success sends a success response with automatic wrapping
func (h *ResponseHelper) Success(c echo.Context, data interface{}, message string) error {
	c.Set("responseData", data)
	c.Set("responseMessage", message)
	return nil
}

// SuccessOnly sends a success response with default message
func (h *ResponseHelper) SuccessOnly(c echo.Context, data interface{}) error {
	c.Set("responseData", data)
	c.Set("responseMessage", "Thành công")
	return nil
}

// Error sends an error response with processing time
func (h *ResponseHelper) Error(c echo.Context, statusCode int, errorResp common.ErrorResponse) error {
	// Get processing time from context
	startTime := c.Get("startTime")
	if startTime != nil {
		if start, ok := startTime.(time.Time); ok {
			processingTime := time.Since(start).Milliseconds()
			errorResp.ProcessingTime = processingTime
		}
	}
	return c.JSON(statusCode, errorResp)
}

// ValidationError sends a validation error response
func (h *ResponseHelper) ValidationError(c echo.Context, message string, details ...common.ErrorDetail) error {
	errorResp := common.ValidationError(message, details...)
	return h.Error(c, http.StatusBadRequest, *errorResp)
}

// InternalError sends an internal server error response
func (h *ResponseHelper) InternalError(c echo.Context, message string) error {
	errorResp := common.InternalError(message)
	return h.Error(c, http.StatusInternalServerError, *errorResp)
}

// NotFound sends a not found error response
func (h *ResponseHelper) NotFound(c echo.Context, message string) error {
	errorResp := common.NotFoundError(message)
	return h.Error(c, http.StatusNotFound, *errorResp)
}

// Unauthorized sends an unauthorized error response
func (h *ResponseHelper) Unauthorized(c echo.Context, message string) error {
	errorResp := common.UnauthorizedError(message)
	return h.Error(c, http.StatusUnauthorized, *errorResp)
}

// BadRequest sends a bad request error response
func (h *ResponseHelper) BadRequest(c echo.Context, message string, details ...common.ErrorDetail) error {
	errorResp := common.ValidationError(message, details...)
	return h.Error(c, http.StatusBadRequest, *errorResp)
}
