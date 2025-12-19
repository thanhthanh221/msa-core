package common

import (
	"net/http"
	"os"
	"strconv"

	"github.com/labstack/echo/v4"
)

// BaseController is a generic base controller for Echo framework
type BaseController[T any] struct{}

// IBaseController interface for base controller methods
type IBaseController[T any] interface {
	Success(c echo.Context, v any) error
	Error(c echo.Context, err *ErrorResponse, v any) error
	ResponseArray(serviceFunc func(c echo.Context) ([]T, *ErrorResponse)) echo.HandlerFunc
	ResponsePage(serviceFunc func(c echo.Context) ([]T, int64, *ErrorResponse)) echo.HandlerFunc
	ResponseObject(serviceFunc func(c echo.Context) (T, *ErrorResponse)) echo.HandlerFunc
	ResponsePointer(serviceFunc func(c echo.Context) (*T, *ErrorResponse)) echo.HandlerFunc
	ResponseSuccessOnly(serviceFunc func(c echo.Context) *ErrorResponse) echo.HandlerFunc
}

// Page represents pagination structure
type Page[E any] struct {
	Content []E   `json:"content"`
	Total   int64 `json:"totalElements"`
}

// Success returns a success response with i18n support
func (controller *BaseController[T]) Success(c echo.Context, v any) error {
	// Get locale from context or header
	locale := GetLocaleFromHeader(c.Request().Header)
	ctx := SetLocaleInContext(c.Request().Context(), locale)

	response := SuccessResponseWithContext(ctx, v, MsgSuccessDefault)
	_ = ctx // Use context to avoid unused variable error
	return c.JSON(http.StatusOK, response)
}

// SuccessWithMessage returns a success response with custom i18n message
func (controller *BaseController[T]) SuccessWithMessage(c echo.Context, v any, messageKey string) error {
	locale := GetLocaleFromHeader(c.Request().Header)
	ctx := SetLocaleInContext(c.Request().Context(), locale)

	response := SuccessResponseWithContext(ctx, v, messageKey)
	_ = ctx // Use context to avoid unused variable error
	return c.JSON(http.StatusOK, response)
}

// SuccessWithPagination returns a success response with pagination and i18n
func (controller *BaseController[T]) SuccessWithPagination(c echo.Context, v any, total int64, page, pageSize int, messageKey string) error {
	locale := GetLocaleFromHeader(c.Request().Header)
	ctx := SetLocaleInContext(c.Request().Context(), locale)

	pagination := CalculatePagination(page, pageSize, total)
	response := SuccessResponseWithPaginationI18n(v, messageKey, pagination)
	_ = ctx // Use context to avoid unused variable error
	return c.JSON(http.StatusOK, response)
}

// Error returns an error response with i18n support
func (controller *BaseController[T]) Error(c echo.Context, err *ErrorResponse, v any) error {
	// Map error code to HTTP status code
	var statusCode int
	codeValue := int(err.Code)
	switch {
	case codeValue == 400: // VALIDATION_ERROR or BAD_REQUEST
		statusCode = http.StatusBadRequest
	case err.Code == NOT_FOUND:
		statusCode = http.StatusNotFound // 404
	case err.Code == UNAUTHORIZED:
		statusCode = http.StatusUnauthorized // 401
	case err.Code == FORBIDDEN:
		statusCode = http.StatusForbidden // 403
	case err.Code == INTERNAL_ERROR:
		statusCode = http.StatusInternalServerError // 500
	case err.Code == CONFLICT:
		statusCode = http.StatusConflict // 409
	default:
		statusCode = http.StatusInternalServerError // 500
	}

	// Get locale from header and translate message
	locale := GetLocaleFromHeader(c.Request().Header)
	ctx := SetLocaleInContext(c.Request().Context(), locale)

	// Set locale for global i18n manager to translate messages
	if manager := GetGlobalI18n(); manager != nil {
		_ = manager.SetLocale(locale)
	}
	_ = ctx // Use context to avoid unused variable error

	// If error has message key, translate it
	var errorResponse ErrorResponse
	if err.Message != "" {
		// Translate the message key to actual message
		errorResponse = *CreateErrorResponseI18n(err.Code, err.Message, err.Details...)
	} else {
		// Otherwise, create appropriate error response based on error code
		switch err.Code {
		case VALIDATION_ERROR:
			errorResponse = *ValidationErrorI18n()
		case NOT_FOUND:
			errorResponse = *NotFoundErrorI18n()
		case UNAUTHORIZED:
			errorResponse = *UnauthorizedErrorI18n()
		case INTERNAL_ERROR:
			errorResponse = *InternalErrorI18n()
		case CONFLICT:
			errorResponse = *ConflictErrorI18n()
		default:
			errorResponse = *err
		}
	}

	// Add custom data if provided
	if v != nil {
		// Create a new error response with custom data
		errorResponse = *CreateErrorResponseI18n(err.Code, err.Message, ErrorDetail{})
	}

	return c.JSON(statusCode, errorResponse)
}

// ErrorWithDetails returns an error response with custom details and i18n
func (controller *BaseController[T]) ErrorWithDetails(ctx echo.Context, code ResponseCode, messageKey string, details ...ErrorDetail) error {
	locale := GetLocaleFromHeader(ctx.Request().Header)
	context := SetLocaleInContext(ctx.Request().Context(), locale)

	// Map error code to HTTP status code
	var statusCode int
	codeValue := int(code)
	switch {
	case codeValue == 400: // VALIDATION_ERROR or BAD_REQUEST
		statusCode = http.StatusBadRequest
	case code == NOT_FOUND:
		statusCode = http.StatusNotFound // 404
	case code == UNAUTHORIZED:
		statusCode = http.StatusUnauthorized // 401
	case code == FORBIDDEN:
		statusCode = http.StatusForbidden // 403
	case code == INTERNAL_ERROR:
		statusCode = http.StatusInternalServerError // 500
	case code == CONFLICT:
		statusCode = http.StatusConflict // 409
	default:
		statusCode = http.StatusInternalServerError // 500
	}

	errorResponse := CreateErrorResponseI18n(code, messageKey, details...)
	_ = context // Use context to avoid unused variable error
	return ctx.JSON(statusCode, errorResponse)
}

// ValidationError returns a validation error response with details
func (controller *BaseController[T]) ValidationError(ctx echo.Context, details ...ErrorDetail) error {
	return controller.ErrorWithDetails(ctx, VALIDATION_ERROR, "response.error.validation", details...)
}

// ValidationErrorWithMessage returns a validation error response with custom message and details
func (controller *BaseController[T]) ValidationErrorWithMessage(ctx echo.Context, messageKey string, details ...ErrorDetail) error {
	return controller.ErrorWithDetails(ctx, VALIDATION_ERROR, messageKey, details...)
}

// HandleValidation handles validation result and returns error if invalid
func (controller *BaseController[T]) HandleValidation(ctx echo.Context, result ValidationResult, messageKey string) error {
	if !result.IsValid {
		return controller.ErrorWithDetails(ctx, VALIDATION_ERROR, messageKey, result.Errors...)
	}
	return nil
}

// HandleBindError handles bind error and returns error response with bind error message in details
func (controller *BaseController[T]) HandleBindError(ctx echo.Context, bindErr error, messageKey string) error {
	// Extract error message from bind error
	var errorDetail ErrorDetail
	if httpErr, ok := bindErr.(*echo.HTTPError); ok {
		// Handle echo.HTTPError
		if msg, ok := httpErr.Message.(string); ok {
			errorDetail = ErrorDetail{
				Field:   "body",
				Message: msg,
			}
		} else {
			errorDetail = ErrorDetail{
				Field:   "body",
				Message: bindErr.Error(),
			}
		}
	} else {
		// Handle regular error
		errorDetail = ErrorDetail{
			Field:   "body",
			Message: bindErr.Error(),
		}
	}
	return controller.ErrorWithDetails(ctx, VALIDATION_ERROR, messageKey, errorDetail)
}

// ResponseArray returns a handler function for array responses
func (controller *BaseController[T]) ResponseArray(serviceFunc func(c echo.Context) ([]T, *ErrorResponse)) echo.HandlerFunc {
	return func(c echo.Context) error {
		body, err := serviceFunc(c)
		if err != nil {
			return controller.Error(c, err, nil)
		}
		return controller.Success(c, body)
	}
}

// ResponseList returns a handler function for list responses with clear data structure
func (controller *BaseController[T]) ResponseList(serviceFunc func(c echo.Context) ([]*T, int64, *ErrorResponse)) echo.HandlerFunc {
	return func(c echo.Context) error {
		body, total, err := serviceFunc(c)
		if err != nil {
			return controller.Error(c, err, nil)
		}

		// Create a structured list response
		listResponse := map[string]interface{}{
			"data":  body,
			"total": total,
		}

		return controller.Success(c, listResponse)
	}
}

// ResponseListWithMessage returns a handler function for list responses with custom message
func (controller *BaseController[T]) ResponseListWithMessage(serviceFunc func(c echo.Context) ([]*T, int64, *ErrorResponse), messageKey string) echo.HandlerFunc {
	return func(c echo.Context) error {
		body, total, err := serviceFunc(c)
		if err != nil {
			return controller.Error(c, err, nil)
		}

		// Create a structured list response
		listResponse := map[string]interface{}{
			"data":  body,
			"total": total,
		}

		return controller.SuccessWithMessage(c, listResponse, messageKey)
	}
}

// ResponseListWithPagination returns a handler function for list responses with pagination
func (controller *BaseController[T]) ResponseListWithPagination(serviceFunc func(c echo.Context) ([]T, int64, *ErrorResponse)) echo.HandlerFunc {
	return func(c echo.Context) error {
		content, total, err := serviceFunc(c)
		if err != nil {
			return controller.Error(c, err, nil)
		}

		// Get pagination parameters from query
		page := 1
		pageSize := 10
		if pageStr := c.QueryParam("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}
		if sizeStr := c.QueryParam("size"); sizeStr != "" {
			if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
				pageSize = s
			}
		}

		// Create pagination info
		pagination := CalculatePagination(page, pageSize, total)

		// Create a structured list response with pagination
		listResponse := map[string]interface{}{
			"data":       content,
			"total":      total,
			"type":       "list",
			"pagination": pagination,
		}

		return controller.SuccessWithPagination(c, listResponse, total, page, pageSize, MsgSuccessRetrieved)
	}
}

// ResponseListWithPaginationSimple returns a handler function for list responses with simple pagination
// Controller only needs to provide data and total, core handles everything else
func (controller *BaseController[T]) ResponseListWithPaginationSimple(serviceFunc func(c echo.Context) ([]T, int64, *ErrorResponse)) echo.HandlerFunc {
	return func(c echo.Context) error {
		content, total, err := serviceFunc(c)
		if err != nil {
			return controller.Error(c, err, nil)
		}

		// Core automatically handles pagination setup and data slicing
		return controller.createPaginationResponse(c, content, total)
	}
}

// ResponseListWithPaginationCustom returns a handler function for list responses with custom pagination
// Controller can specify custom page and size, core handles the rest
func (controller *BaseController[T]) ResponseListWithPaginationCustom(serviceFunc func(c echo.Context) ([]T, int64, *ErrorResponse), defaultPageSize int) echo.HandlerFunc {
	return func(c echo.Context) error {
		content, total, err := serviceFunc(c)
		if err != nil {
			return controller.Error(c, err, nil)
		}

		// Core automatically handles pagination setup with custom default page size
		return controller.createPaginationResponseWithCustomSize(c, content, total, defaultPageSize)
	}
}

// ResponseListWithPaginationAuto returns a handler function that automatically applies pagination to data
// Controller provides all data, core automatically slices and paginates
func (controller *BaseController[T]) ResponseListWithPaginationAuto(serviceFunc func(c echo.Context) ([]T, *ErrorResponse)) echo.HandlerFunc {
	return func(c echo.Context) error {
		content, err := serviceFunc(c)
		if err != nil {
			return controller.Error(c, err, nil)
		}

		// Core automatically calculates total and applies pagination
		total := int64(len(content))
		return controller.createPaginationResponseWithDataSlicing(c, content, total)
	}
}

// ResponseListWithPaginationAutoDB returns a handler function that works with database pagination
// Controller provides paginated data and total count from database
func (controller *BaseController[T]) ResponseListWithPaginationAutoDB(serviceFunc func(c echo.Context) ([]*T, int64, *ErrorResponse)) echo.HandlerFunc {
	return func(c echo.Context) error {
		content, total, err := serviceFunc(c)
		if err != nil {
			return controller.Error(c, err, nil)
		}

		// Core receives paginated data and total from database
		// No need to slice data again, just create response structure
		return controller.createPaginationResponseFromDB(c, content, total)
	}
}

// ResponseListWithPaginationAutoDBAndSorting returns a handler function with database pagination and sorting
func (controller *BaseController[T]) ResponseListWithPaginationAutoDBAndSorting(serviceFunc func(c echo.Context) ([]*T, int64, *ErrorResponse)) echo.HandlerFunc {
	return func(c echo.Context) error {
		content, total, err := serviceFunc(c)
		if err != nil {
			return controller.Error(c, err, nil)
		}

		// Core receives paginated data and total from database with sorting info
		return controller.createPaginationResponseFromDBWithSorting(c, content, total)
	}
}

// ResponseListWithPaginationAndSorting returns a handler function with automatic pagination and sorting
func (controller *BaseController[T]) ResponseListWithPaginationAndSorting(serviceFunc func(c echo.Context) ([]T, int64, *ErrorResponse)) echo.HandlerFunc {
	return func(c echo.Context) error {
		content, total, err := serviceFunc(c)
		if err != nil {
			return controller.Error(c, err, nil)
		}

		// Core automatically calculates total, applies sorting and pagination
		return controller.createPaginationResponseWithSortingAndSlicing(c, content, total)
	}
}

// ResponsePage returns a handler function for paginated responses
func (controller *BaseController[T]) ResponsePage(serviceFunc func(c echo.Context) ([]T, int64, *ErrorResponse)) echo.HandlerFunc {
	return func(c echo.Context) error {
		content, total, err := serviceFunc(c)
		if err != nil {
			return controller.Error(c, err, nil)
		}

		// Get pagination parameters from query
		page := 1
		pageSize := 10
		if pageStr := c.QueryParam("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}
		if sizeStr := c.QueryParam("size"); sizeStr != "" {
			if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
				pageSize = s
			}
		}

		return controller.SuccessWithPagination(c, content, total, page, pageSize, MsgSuccessRetrieved)
	}
}

// ResponseObject returns a handler function for object responses
func (controller *BaseController[T]) ResponseObject(serviceFunc func(c echo.Context) (T, *ErrorResponse)) echo.HandlerFunc {
	return func(c echo.Context) error {
		body, err := serviceFunc(c)
		if err != nil {
			return controller.Error(c, err, nil)
		}
		return controller.Success(c, body)
	}
}

// ResponsePointer returns a handler function for pointer responses
func (controller *BaseController[T]) ResponsePointer(serviceFunc func(c echo.Context) (*T, *ErrorResponse)) echo.HandlerFunc {
	return func(c echo.Context) error {
		body, err := serviceFunc(c)
		if err != nil {
			return controller.Error(c, err, nil)
		}
		return controller.Success(c, body)
	}
}

// ResponseSuccessOnly returns a handler function for success-only responses
func (controller *BaseController[T]) ResponseSuccessOnly(serviceFunc func(c echo.Context) *ErrorResponse) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := serviceFunc(c)
		if err != nil {
			return controller.Error(c, err, nil)
		}
		return controller.Success(c, nil)
	}
}

// ResponseArrayWithMessage returns a handler function for array responses with custom message
func (controller *BaseController[T]) ResponseArrayWithMessage(serviceFunc func(c echo.Context) ([]T, *ErrorResponse), messageKey string) echo.HandlerFunc {
	return func(c echo.Context) error {
		body, err := serviceFunc(c)
		if err != nil {
			return controller.Error(c, err, nil)
		}
		return controller.SuccessWithMessage(c, body, messageKey)
	}
}

// ResponseObjectWithMessage returns a handler function for object responses with custom message
func (controller *BaseController[T]) ResponseObjectWithMessage(serviceFunc func(c echo.Context) (T, *ErrorResponse), messageKey string) echo.HandlerFunc {
	return func(c echo.Context) error {
		body, err := serviceFunc(c)
		if err != nil {
			return controller.Error(c, err, nil)
		}
		return controller.SuccessWithMessage(c, body, messageKey)
	}
}

// ResponseCreated returns a success response for resource creation
func (controller *BaseController[T]) ResponseCreated(c echo.Context, v any) error {
	return controller.SuccessWithMessage(c, v, MsgSuccessCreated)
}

// ResponseUpdated returns a success response for resource updates
func (controller *BaseController[T]) ResponseUpdated(c echo.Context, v any) error {
	return controller.SuccessWithMessage(c, v, MsgSuccessUpdated)
}

// ResponseDeleted returns a success response for resource deletion
func (controller *BaseController[T]) ResponseDeleted(c echo.Context) error {
	return controller.SuccessWithMessage(c, nil, MsgSuccessDeleted)
}

// ResponseRetrieved returns a success response for resource retrieval
func (controller *BaseController[T]) ResponseRetrieved(c echo.Context, v any) error {
	return controller.SuccessWithMessage(c, v, MsgSuccessRetrieved)
}

// createPaginationResponse automatically creates pagination response
func (controller *BaseController[T]) createPaginationResponse(c echo.Context, content []T, total int64) error {
	// Get pagination parameters from query with smart defaults
	page := controller.getPageFromQuery(c)
	pageSize := controller.getPageSizeFromQuery(c)

	// Create pagination info
	pagination := CalculatePagination(page, pageSize, total)

	// Create structured response
	response := map[string]interface{}{
		"data":       content,
		"total":      total,
		"pagination": pagination,
		"meta": map[string]interface{}{
			"current_page": page,
			"page_size":    pageSize,
			"total_pages":  pagination.TotalPages,
			"has_next":     pagination.HasNext,
			"has_prev":     pagination.HasPrev,
		},
	}

	return controller.SuccessWithMessage(c, response, MsgSuccessRetrieved)
}

// createPaginationResponseWithCustomSize creates pagination response with custom default page size
func (controller *BaseController[T]) createPaginationResponseWithCustomSize(c echo.Context, content []T, total int64, defaultPageSize int) error {
	// Get pagination parameters from query with custom default
	page := controller.getPageFromQuery(c)
	pageSize := controller.getPageSizeFromQueryWithDefault(c, defaultPageSize)

	// Create pagination info
	pagination := CalculatePagination(page, pageSize, total)

	// Create structured response
	response := map[string]interface{}{
		"data":       content,
		"total":      total,
		"pagination": pagination,
		"meta": map[string]interface{}{
			"current_page": page,
			"page_size":    pageSize,
			"total_pages":  pagination.TotalPages,
			"has_next":     pagination.HasNext,
			"has_prev":     pagination.HasPrev,
		},
	}

	return controller.SuccessWithMessage(c, response, MsgSuccessRetrieved)
}

// createPaginationResponseWithDataSlicing creates pagination response with data slicing
func (controller *BaseController[T]) createPaginationResponseWithDataSlicing(c echo.Context, content []T, total int64) error {
	// Get pagination parameters from query with smart defaults
	page := controller.getPageFromQuery(c)
	pageSize := controller.getPageSizeFromQuery(c)

	// Slice data based on pagination
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > int(total) {
		end = int(total)
	}
	paginatedContent := content[start:end]

	// Create pagination info
	pagination := CalculatePagination(page, pageSize, total)

	// Create structured response
	response := map[string]interface{}{
		"data":       paginatedContent,
		"total":      total,
		"pagination": pagination,
		"meta": map[string]interface{}{
			"current_page": page,
			"page_size":    pageSize,
			"total_pages":  pagination.TotalPages,
			"has_next":     pagination.HasNext,
			"has_prev":     pagination.HasPrev,
		},
	}

	return controller.SuccessWithMessage(c, response, MsgSuccessRetrieved)
}

// createPaginationResponseFromDB creates pagination response from database-paginated data
func (controller *BaseController[T]) createPaginationResponseFromDB(c echo.Context, content []*T, total int64) error {
	// Get pagination parameters from query
	page := controller.getPageFromQuery(c)
	pageSize := controller.getPageSizeFromQuery(c)

	// Data is already paginated from database, just create response structure
	pagination := CalculatePagination(page, pageSize, total)

	// Create structured response
	response := map[string]interface{}{
		"data":       content, // Data đã được paginate từ DB
		"total":      total,   // Total count từ DB
		"pagination": pagination,
		"meta": map[string]interface{}{
			"current_page": page,
			"page_size":    pageSize,
			"total_pages":  pagination.TotalPages,
			"has_next":     pagination.HasNext,
			"has_prev":     pagination.HasPrev,
		},
	}

	return controller.SuccessWithMessage(c, response, MsgSuccessRetrieved)
}

// createPaginationResponseFromDBWithSorting creates pagination response from database with sorting info
func (controller *BaseController[T]) createPaginationResponseFromDBWithSorting(c echo.Context, content []*T, total int64) error {
	// Get pagination and sorting parameters from query
	page := controller.getPageFromQuery(c)
	pageSize := controller.getPageSizeFromQuery(c)
	sortBy, sortOrder := controller.getSortingFromQuery(c)

	// Data is already paginated and sorted from database, just create response structure
	pagination := CalculatePagination(page, pageSize, total)

	// Create structured response with sorting info
	response := map[string]interface{}{
		"data":       content, // Data đã được paginate và sort từ DB
		"total":      total,   // Total count từ DB
		"pagination": pagination,
		"sorting": map[string]interface{}{
			"sort_by":    sortBy,
			"sort_order": sortOrder,
		},
		"meta": map[string]interface{}{
			"current_page": page,
			"page_size":    pageSize,
			"total_pages":  pagination.TotalPages,
			"has_next":     pagination.HasNext,
			"has_prev":     pagination.HasPrev,
		},
	}

	return controller.SuccessWithMessage(c, response, MsgSuccessRetrieved)
}

// createPaginationResponseWithSortingAndSlicing creates pagination response with sorting and data slicing
func (controller *BaseController[T]) createPaginationResponseWithSortingAndSlicing(c echo.Context, content []T, total int64) error {
	// Get pagination and sorting parameters from query
	page := controller.getPageFromQuery(c)
	pageSize := controller.getPageSizeFromQuery(c)
	sortBy, sortOrder := controller.getSortingFromQuery(c)

	// Apply sorting if specified (basic string sorting for demonstration)
	// In real implementation, you might want to use reflection or custom sorting
	sortedContent := controller.applyBasicSorting(content)

	// Slice data based on pagination
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > int(total) {
		end = int(total)
	}

	// Check bounds to prevent panic when content is empty or start exceeds length
	var paginatedContent []T
	if start >= len(sortedContent) || len(sortedContent) == 0 {
		// Return empty slice if start is beyond content length or content is empty
		paginatedContent = make([]T, 0)
	} else {
		if end > len(sortedContent) {
			end = len(sortedContent)
		}
		paginatedContent = sortedContent[start:end]
	}

	// Create pagination info
	pagination := CalculatePagination(page, pageSize, total)

	// Create structured response with sorting info
	response := map[string]interface{}{
		"data":       paginatedContent,
		"total":      total,
		"pagination": pagination,
		"sorting": map[string]interface{}{
			"sort_by":    sortBy,
			"sort_order": sortOrder,
		},
		"meta": map[string]interface{}{
			"current_page": page,
			"page_size":    pageSize,
			"total_pages":  pagination.TotalPages,
			"has_next":     pagination.HasNext,
			"has_prev":     pagination.HasPrev,
		},
	}

	return controller.SuccessWithMessage(c, response, MsgSuccessRetrieved)
}

// applyBasicSorting applies basic sorting to content (placeholder for demonstration)
// In real implementation, you would implement proper sorting logic
func (controller *BaseController[T]) applyBasicSorting(content []T) []T {
	// For now, return content as-is
	// In real implementation, you would use reflection or custom sorting logic
	// based on the sortBy field and sortOrder
	return content
}

// getSortingFromQuery extracts sorting parameters from query
func (controller *BaseController[T]) getSortingFromQuery(c echo.Context) (string, string) {
	sortBy := c.QueryParam("sort_by")

	// Try to get sort_order first (for compatibility)
	sortOrder := c.QueryParam("sort_order")

	// If sort_order is not provided, try to get from desc parameter (boolean)
	if sortOrder == "" {
		descParam := c.QueryParam("desc")
		switch descParam {
		case "true":
			sortOrder = "desc"
		case "false":
			sortOrder = "asc"
		}
	}

	// Default sorting
	if sortBy == "" {
		sortBy = "created_at"
	}
	if sortOrder == "" || (sortOrder != "asc" && sortOrder != "desc") {
		sortOrder = "desc"
	}

	return sortBy, sortOrder
}

// getPageFromQuery extracts page from query parameters with smart defaults
func (controller *BaseController[T]) getPageFromQuery(c echo.Context) int {
	page := 1
	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	return page
}

// getPageSizeFromQuery extracts page size from query parameters with smart defaults
func (controller *BaseController[T]) getPageSizeFromQuery(c echo.Context) int {
	pageSize := 10 // Default page size
	if sizeStr := c.QueryParam("size"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
			// Limit maximum page size to prevent abuse
			if s > 100 {
				s = 100
			}
			pageSize = s
		}
	}
	return pageSize
}

// getPageSizeFromQueryWithDefault extracts page size with custom default
func (controller *BaseController[T]) getPageSizeFromQueryWithDefault(c echo.Context, defaultSize int) int {
	pageSize := defaultSize
	if sizeStr := c.QueryParam("size"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
			// Limit maximum page size to prevent abuse
			if s > 100 {
				s = 100
			}
			pageSize = s
		}
	}
	return pageSize
}

// FileResponse returns a file response with proper headers
func (controller *BaseController[T]) FileResponse(c echo.Context, filePath, fileName string) error {
	return c.Attachment(filePath, fileName)
}

// FileStream returns a file stream response
func (controller *BaseController[T]) FileStream(c echo.Context, filePath, fileName, contentType string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return c.Stream(http.StatusOK, contentType, file)
}

// FileDownload returns a file download response with custom filename
func (controller *BaseController[T]) FileDownload(c echo.Context, filePath, fileName string) error {
	c.Response().Header().Set("Content-Disposition", "attachment; filename="+fileName)
	return c.File(filePath)
}
