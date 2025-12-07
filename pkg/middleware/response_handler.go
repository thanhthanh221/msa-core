package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/thanhthanh221/msa-core/pkg/common"
)

// ResponseWrapper wraps the response in common base response format
type ResponseWrapper struct {
	Data any `json:"data"`
}

// ResponseHandlerMiddleware automatically wraps responses in common base response format
// and measures processing time
func ResponseHandlerMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			startTime := time.Now()

			// Store start time in context for potential use in handlers
			c.Set("startTime", startTime)

			// Call the next handler
			err := next(c)

			// Calculate processing time
			processingTime := time.Since(startTime).Milliseconds()

			// If there's an error, it's already handled by ErrorHandlerMiddleware
			if err != nil {
				return err
			}

			// Get the response status
			status := c.Response().Status

			// If status is not 200, don't wrap (let ErrorHandlerMiddleware handle it)
			if status != http.StatusOK {
				return nil
			}

			// Get the response data from context
			responseData := c.Get("responseData")
			responseMessage := c.Get("responseMessage")

			// If no response data is set, don't wrap
			if responseData == nil {
				return nil
			}

			// Create the base response
			message := "Thành công"
			if responseMessage != nil {
				if msg, ok := responseMessage.(string); ok {
					message = msg
				}
			}

			baseResponse := common.SuccessResponse(responseData, message)
			baseResponse.ProcessingTime = processingTime

			// Return the wrapped response
			return c.JSON(http.StatusOK, baseResponse)
		}
	}
}

// SetResponseData sets the response data and message in the context
// This should be called in controllers before returning
func SetResponseData(c echo.Context, data any, message string) {
	c.Set("responseData", data)
	c.Set("responseMessage", message)
}

// SetResponseDataOnly sets only the response data (uses default message)
func SetResponseDataOnly(c echo.Context, data any) {
	c.Set("responseData", data)
}

// GetProcessingTime returns the processing time in milliseconds
func GetProcessingTime(c echo.Context) int64 {
	startTime := c.Get("startTime")
	if startTime == nil {
		return 0
	}

	if start, ok := startTime.(time.Time); ok {
		return time.Since(start).Milliseconds()
	}

	return 0
}

// ErrorHandlerMiddleware handles errors and converts them to standard error responses
func ErrorHandlerMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)

			if err != nil {
				// Get processing time from header
				processingTimeStr := c.Response().Header().Get("X-Processing-Time")
				processingTime := int64(0)
				if processingTimeStr != "" {
					// Convert string to int64 if possible
					if pt, err := strconv.ParseInt(processingTimeStr, 10, 64); err == nil {
						processingTime = pt
					}
				}

				// Handle different types of errors
				switch e := err.(type) {
				case *echo.HTTPError:
					// Convert HTTP status code to ResponseCode
					errorResp := common.CreateErrorResponse(
						common.ResponseCode(e.Code),
						e.Message.(string),
					)
					errorResp.ProcessingTime = processingTime
					return c.JSON(e.Code, errorResp)
				default:
					// Handle unknown errors
					errorResp := common.InternalError("Lỗi hệ thống không xác định")
					errorResp.ProcessingTime = processingTime
					return c.JSON(http.StatusInternalServerError, errorResp)
				}
			}

			return nil
		}
	}
}

// ValidationErrorHandler handles validation errors specifically
func ValidationErrorHandler() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)

			if err != nil {
				// Check if it's a validation error
				if httpError, ok := err.(*echo.HTTPError); ok && httpError.Code == http.StatusBadRequest {
					processingTime := int64(0)

					errorResp := common.ValidationError("Dữ liệu không hợp lệ")
					errorResp.ProcessingTime = processingTime
					return c.JSON(http.StatusBadRequest, errorResp)
				}
			}

			return err
		}
	}
}
