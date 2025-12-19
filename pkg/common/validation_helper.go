package common

import (
	"reflect"
	"strconv"
	"strings"
)

// ValidationRule represents a validation rule
type ValidationRule struct {
	Field   string
	Rule    string
	Message string
	Value   interface{}
}

// ValidationResult represents the result of validation
type ValidationResult struct {
	IsValid bool
	Errors  []ErrorDetail
}

// Validator interface for validation operations
type Validator interface {
	Validate() ValidationResult
}

// BaseValidator provides common validation functionality
type BaseValidator struct{}

// Required validates that a field is not empty
func (v *BaseValidator) Required(field string, value interface{}, messageKey string) *ErrorDetail {
	if isEmpty(value) {
		return &ErrorDetail{
			Field:   field,
			Message: T(messageKey),
			Value:   toString(value),
		}
	}
	return nil
}

// MinLength validates minimum length for strings
func (v *BaseValidator) MinLength(field string, value string, minLength int, messageKey string) *ErrorDetail {
	if len(value) < minLength {
		return &ErrorDetail{
			Field:   field,
			Message: T(messageKey),
			Value:   value,
		}
	}
	return nil
}

// MaxLength validates maximum length for strings
func (v *BaseValidator) MaxLength(field string, value string, maxLength int, messageKey string) *ErrorDetail {
	if len(value) > maxLength {
		return &ErrorDetail{
			Field:   field,
			Message: T(messageKey),
			Value:   value,
		}
	}
	return nil
}

// MinValue validates minimum value for numbers
func (v *BaseValidator) MinValue(field string, value float64, minValue float64, messageKey string) *ErrorDetail {
	if value < minValue {
		return &ErrorDetail{
			Field:   field,
			Message: T(messageKey),
			Value:   strconv.FormatFloat(value, 'f', -1, 64),
		}
	}
	return nil
}

// MaxValue validates maximum value for numbers
func (v *BaseValidator) MaxValue(field string, value float64, maxValue float64, messageKey string) *ErrorDetail {
	if value > maxValue {
		return &ErrorDetail{
			Field:   field,
			Message: T(messageKey),
			Value:   strconv.FormatFloat(value, 'f', -1, 64),
		}
	}
	return nil
}

// Email validates email format
func (v *BaseValidator) Email(field string, value string, messageKey string) *ErrorDetail {
	if !isValidEmail(value) {
		return &ErrorDetail{
			Field:   field,
			Message: T(messageKey),
			Value:   value,
		}
	}
	return nil
}

// URL validates URL format
func (v *BaseValidator) URL(field string, value string, messageKey string) *ErrorDetail {
	if !isValidURL(value) {
		return &ErrorDetail{
			Field:   field,
			Message: T(messageKey),
			Value:   value,
		}
	}
	return nil
}

// Custom validates with custom validation function
func (v *BaseValidator) Custom(field string, value interface{}, validator func(interface{}) bool, messageKey string) *ErrorDetail {
	if !validator(value) {
		return &ErrorDetail{
			Field:   field,
			Message: T(messageKey),
			Value:   toString(value),
		}
	}
	return nil
}

// ValidateMultiple validates multiple rules and returns all errors
func (v *BaseValidator) ValidateMultiple(rules ...*ErrorDetail) ValidationResult {
	var errors []ErrorDetail
	isValid := true

	for _, rule := range rules {
		if rule != nil {
			errors = append(errors, *rule)
			isValid = false
		}
	}

	return ValidationResult{
		IsValid: isValid,
		Errors:  errors,
	}
}

// Helper functions
func isEmpty(value interface{}) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return strings.TrimSpace(v.String()) == ""
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}

func toString(value interface{}) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(reflect.ValueOf(value).String())
}

func isValidEmail(email string) bool {
	// Basic email validation
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

func isValidURL(url string) bool {
	// Basic URL validation
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// Common validation message keys
const (
	MsgValidationRequired  = "validation.required"
	MsgValidationMinLength = "validation.min_length"
	MsgValidationMaxLength = "validation.max_length"
	MsgValidationMinValue  = "validation.min_value"
	MsgValidationMaxValue  = "validation.max_value"
	MsgValidationEmail     = "validation.email"
	MsgValidationURL       = "validation.url"
	MsgValidationInvalid   = "validation.invalid"
)
