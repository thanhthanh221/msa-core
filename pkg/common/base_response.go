package common

import (
	"context"
	"time"
)

// ResponseCode represents standard response codes as integers
type ResponseCode int

const (
	// Success codes
	SUCCESS ResponseCode = 200

	// Error codes
	VALIDATION_ERROR    ResponseCode = 400
	UNAUTHORIZED        ResponseCode = 401
	FORBIDDEN           ResponseCode = 403
	NOT_FOUND           ResponseCode = 404
	CONFLICT            ResponseCode = 409
	INTERNAL_ERROR      ResponseCode = 500
	BAD_REQUEST         ResponseCode = 400
	METHOD_NOT_ALLOWED  ResponseCode = 405
	REQUEST_TIMEOUT     ResponseCode = 408
	TOO_MANY_REQUESTS   ResponseCode = 429
	SERVICE_UNAVAILABLE ResponseCode = 503
)

// BaseResponse represents the standard response structure
// @Description Phản hồi chuẩn của API
type BaseResponse struct {
	// @Description Mã phản hồi (số nguyên)
	// @example 200
	Code ResponseCode `json:"code" example:"200" swaggertype:"integer"`

	// @Description Thông báo phản hồi
	// @example "Thao tác thành công"
	Message string `json:"message" example:"Thao tác thành công"`

	// @Description Dữ liệu trả về
	Data interface{} `json:"data,omitempty"`

	// @Description Thông tin phân trang (nếu có)
	Pagination *PaginationInfo `json:"pagination,omitempty"`

	// @Description Thời gian phản hồi
	// @example "2024-01-15T10:30:00Z"
	Timestamp time.Time `json:"timestamp" example:"2024-01-15T10:30:00Z"`

	// @Description Thời gian xử lý request (milliseconds)
	// @example 150
	ProcessingTime int64 `json:"processing_time,omitempty" example:"150"`
}

// PaginationInfo represents pagination information
// @Description Thông tin phân trang
type PaginationInfo struct {
	// @Description Trang hiện tại
	// @example 1
	CurrentPage int `json:"current_page" example:"1"`

	// @Description Số lượng item trên mỗi trang
	// @example 10
	PageSize int `json:"page_size" example:"10"`

	// @Description Tổng số trang
	// @example 50
	TotalPages int `json:"total_pages" example:"50"`

	// @Description Tổng số item
	// @example 500
	TotalItems int64 `json:"total_items" example:"500"`

	// @Description Có trang tiếp theo không
	// @example true
	HasNext bool `json:"has_next" example:"true"`

	// @Description Có trang trước không
	// @example false
	HasPrev bool `json:"has_prev" example:"false"`
}

// ErrorDetail represents detailed error information
// @Description Chi tiết lỗi
type ErrorDetail struct {
	// @Description Trường bị lỗi
	// @example "email"
	Field string `json:"field,omitempty" example:"email"`

	// @Description Mô tả lỗi cho trường
	// @example "Email không hợp lệ"
	Message string `json:"message" example:"Email không hợp lệ"`

	// @Description Giá trị không hợp lệ
	// @example "invalid-email"
	Value string `json:"value,omitempty" example:"invalid-email"`
}

// ErrorResponse represents error response structure
// @Description Phản hồi lỗi
type ErrorResponse struct {
	// @Description Mã lỗi (số nguyên)
	// @example 400
	Code ResponseCode `json:"code" example:"400" swaggertype:"integer"`

	// @Description Thông báo lỗi
	// @example "Dữ liệu không hợp lệ"
	Message string `json:"message" example:"Dữ liệu không hợp lệ"`

	// @Description Chi tiết lỗi (nếu có)
	Details []ErrorDetail `json:"details,omitempty"`

	// @Description Thời gian xảy ra lỗi
	// @example "2024-01-15T10:30:00Z"
	Timestamp time.Time `json:"timestamp" example:"2024-01-15T10:30:00Z"`

	// @Description Thời gian xử lý request (milliseconds)
	// @example 50
	ProcessingTime int64 `json:"processing_time,omitempty" example:"50"`
}

// SuccessResponse creates a success response
func SuccessResponse(data interface{}, message string) BaseResponse {
	return BaseResponse{
		Code:      SUCCESS,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// SuccessResponseI18n creates a success response with i18n message
func SuccessResponseI18n(data interface{}, messageKey string) BaseResponse {
	return BaseResponse{
		Code:      SUCCESS,
		Message:   T(messageKey),
		Data:      data,
		Timestamp: time.Now(),
	}
}

// SuccessResponseWithContext creates a success response with context-based i18n
func SuccessResponseWithContext(ctx context.Context, data interface{}, messageKey string) BaseResponse {
	return BaseResponse{
		Code:      SUCCESS,
		Message:   TWithContext(ctx, messageKey),
		Data:      data,
		Timestamp: time.Now(),
	}
}

// SuccessResponseWithPagination creates a success response with pagination
func SuccessResponseWithPagination(data interface{}, message string, pagination PaginationInfo) BaseResponse {
	return BaseResponse{
		Code:       SUCCESS,
		Message:    message,
		Pagination: &pagination,
		Timestamp:  time.Now(),
	}
}

// SuccessResponseWithPaginationI18n creates a success response with pagination and i18n message
func SuccessResponseWithPaginationI18n(data interface{}, messageKey string, pagination PaginationInfo) BaseResponse {
	return BaseResponse{
		Code:       SUCCESS,
		Message:    T(messageKey),
		Data:       data,
		Pagination: &pagination,
		Timestamp:  time.Now(),
	}
}

// ErrorResponse creates an error response
func CreateErrorResponse(code ResponseCode, message string, details ...ErrorDetail) *ErrorResponse {
	return &ErrorResponse{
		Code:      code,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// CreateErrorResponseI18n creates an error response with i18n message
func CreateErrorResponseI18n(code ResponseCode, messageKey string, details ...ErrorDetail) *ErrorResponse {
	return &ErrorResponse{
		Code:      code,
		Message:   T(messageKey),
		Details:   details,
		Timestamp: time.Now(),
	}
}

// ValidationError creates a validation error response
func ValidationError(message string, details ...ErrorDetail) *ErrorResponse {
	return CreateErrorResponse(VALIDATION_ERROR, message, details...)
}

// ValidationErrorI18n creates a validation error response with i18n message
func ValidationErrorI18n(details ...ErrorDetail) *ErrorResponse {
	return CreateErrorResponseI18n(VALIDATION_ERROR, "response.error.validation", details...)
}

// NotFoundError creates a not found error response
func NotFoundError(message string) *ErrorResponse {
	return CreateErrorResponse(NOT_FOUND, message)
}

// NotFoundErrorI18n creates a not found error response with i18n message
func NotFoundErrorI18n() *ErrorResponse {
	return CreateErrorResponseI18n(NOT_FOUND, "response.error.not_found")
}

// UnauthorizedError creates an unauthorized error response
func UnauthorizedError(message string) *ErrorResponse {
	return CreateErrorResponse(UNAUTHORIZED, message)
}

// UnauthorizedErrorI18n creates an unauthorized error response with i18n message
func UnauthorizedErrorI18n() *ErrorResponse {
	return CreateErrorResponseI18n(UNAUTHORIZED, "response.error.unauthorized")
}

// InternalError creates an internal error response
func InternalError(message string) *ErrorResponse {
	return CreateErrorResponse(INTERNAL_ERROR, message)
}

// InternalErrorI18n creates an internal error response with i18n message
func InternalErrorI18n() *ErrorResponse {
	return CreateErrorResponseI18n(INTERNAL_ERROR, "response.error.internal")
}

// BadRequestError creates a bad request error response
func BadRequestError(message string) *ErrorResponse {
	return CreateErrorResponse(BAD_REQUEST, message)
}

// BadRequestErrorI18n creates a bad request error response with i18n message
func BadRequestErrorI18n() *ErrorResponse {
	return CreateErrorResponseI18n(BAD_REQUEST, "response.error.bad_request")
}

// ConflictError creates a conflict error response
func ConflictError(message string) *ErrorResponse {
	return CreateErrorResponse(CONFLICT, message)
}

// ConflictErrorI18n creates a conflict error response with i18n message
func ConflictErrorI18n() *ErrorResponse {
	return CreateErrorResponseI18n(CONFLICT, "response.error.conflict")
}

// CalculatePagination calculates pagination information
func CalculatePagination(currentPage, pageSize int, totalItems int64) PaginationInfo {
	totalPages := int((totalItems + int64(pageSize) - 1) / int64(pageSize))

	return PaginationInfo{
		CurrentPage: currentPage,
		PageSize:    pageSize,
		TotalPages:  totalPages,
		TotalItems:  totalItems,
		HasNext:     currentPage < totalPages,
		HasPrev:     currentPage > 1,
	}
}

// AuthResponse represents authentication response
// @Description Phản hồi xác thực
type AuthResponse struct {
	// @Description Access token
	// @example "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
	AccessToken string `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`

	// @Description Refresh token
	// @example "def50200..."
	RefreshToken string `json:"refresh_token" example:"def50200..."`

	// @Description Token type
	// @example "Bearer"
	TokenType string `json:"token_type" example:"Bearer"`

	// @Description Expires in seconds
	// @example 3600
	ExpiresIn int `json:"expires_in" example:"3600"`

	// @Description JWT token for application
	// @example "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
	JWTToken string `json:"jwt_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`

	// @Description User information
	User any `json:"user"`
}

// LogoutResponse represents logout response
// @Description Phản hồi đăng xuất
type LogoutResponse struct {
	// @Description Logout message
	// @example "Logged out successfully"
	Message string `json:"message" example:"Logged out successfully"`
}

// RefreshTokenRequest represents refresh token request
// @Description Yêu cầu refresh token
type RefreshTokenRequest struct {
	// @Description Refresh token
	// @example "def50200..."
	RefreshToken string `json:"refresh_token" validate:"required" example:"def50200..."`
}

// ProviderInfo represents OAuth2 provider information
// @Description Thông tin OAuth2 provider
type ProviderInfo struct {
	// @Description Provider name
	// @example "MSA Backend OAuth2 Provider"
	Name string `json:"name" example:"MSA Backend OAuth2 Provider"`

	// @Description Provider version
	// @example "1.0.0"
	Version string `json:"version" example:"1.0.0"`

	// @Description Provider description
	// @example "OAuth2 Authorization Server for MSA Backend"
	Description string `json:"description" example:"OAuth2 Authorization Server for MSA Backend"`

	// @Description Available endpoints
	Endpoints map[string]string `json:"endpoints"`

	// @Description Supported OAuth2 flows
	// @example ["authorization_code","implicit","client_credentials","refresh_token"]
	SupportedFlows []string `json:"supported_flows" example:"authorization_code,implicit,client_credentials,refresh_token"`

	// @Description Supported scopes
	// @example ["read","write","admin","openid","profile","email"]
	SupportedScopes []string `json:"supported_scopes" example:"read,write,admin,openid,profile,email"`

	// @Description Number of registered clients
	// @example 3
	ClientsCount int `json:"clients_count" example:"3"`
}

// ClientListResponse represents list of OAuth2 clients
// @Description Danh sách OAuth2 clients
type ClientListResponse struct {
	// @Description List of OAuth2 clients
	Clients []any `json:"clients"`
}

// ClientCreateResponse represents OAuth2 client creation response
// @Description Phản hồi tạo OAuth2 client
type ClientCreateResponse struct {
	// @Description Success message
	// @example "Client registered successfully"
	Message string `json:"message" example:"Client registered successfully"`

	// @Description Created client information
	Client any `json:"client"`
}
