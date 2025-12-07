package common

import (
	"context"
	"net/http"
	"strings"
)

// I18nContextKey is the key used to store locale in context
type contextKey string

const I18nContextKey contextKey = "locale"

// GetLocaleFromContext extracts locale from context
func GetLocaleFromContext(ctx context.Context) string {
	if locale, ok := ctx.Value(I18nContextKey).(string); ok {
		return locale
	}
	return "vn" // default locale (Vietnamese)
}

// SetLocaleInContext sets locale in context
func SetLocaleInContext(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, I18nContextKey, locale)
}

// GetLocaleFromHeader extracts locale from Accept-Language header
// Supports quality values: "en-US,vi;q=0.9,en;q=0.8"
func GetLocaleFromHeader(header http.Header) string {
	acceptLang := header.Get("Accept-Language")
	if acceptLang == "" {
		return "vn" // default locale (Vietnamese)
	}

	// Parse Accept-Language header with quality values
	// Format: "en-US,vi;q=0.9,en;q=0.8,fr-FR;q=0.7,fr;q=0.6"
	// Split by comma to get all language preferences
	langs := strings.Split(acceptLang, ",")

	// Supported locales in the system
	supportedLocales := map[string]string{
		"en": "en",
		"vn": "vn",
		"vi": "vn",
	}

	// Try to find first supported locale
	for _, lang := range langs {
		// Remove whitespace
		lang = strings.TrimSpace(lang)

		// Split by semicolon to get language and quality value
		parts := strings.Split(lang, ";")
		langCode := strings.TrimSpace(parts[0])

		// Remove region code if present (e.g., "vi-VN" -> "vi", "en-US" -> "en")
		if idx := strings.Index(langCode, "-"); idx > 0 {
			langCode = langCode[:idx]
		}

		// Normalize to lowercase
		langCode = strings.ToLower(langCode)

		// Check if this locale is supported
		if mappedLocale, ok := supportedLocales[langCode]; ok {
			return mappedLocale
		}
	}

	// If no supported locale found, return default
	return "vn" // default locale (Vietnamese)
}

// TWithContext gets a message using locale from context
func TWithContext(ctx context.Context, keyPath string) string {
	locale := GetLocaleFromContext(ctx)
	globalI18n := GetGlobalI18n()
	if globalI18n != nil && locale != globalI18n.GetLocale() {
		// Create a new i18n manager for this locale
		if manager, err := NewI18nManager(locale); err == nil {
			return manager.GetMessage(keyPath)
		}
	}
	if globalI18n != nil {
		return T(keyPath)
	}
	// Fallback if i18n is not initialized
	return keyPath
}

// TWithContextAndFallback gets a message using locale from context with fallback
func TWithContextAndFallback(ctx context.Context, keyPath string, fallback string) string {
	locale := GetLocaleFromContext(ctx)
	globalI18n := GetGlobalI18n()
	if globalI18n != nil && locale != globalI18n.GetLocale() {
		// Create a new i18n manager for this locale
		if manager, err := NewI18nManager(locale); err == nil {
			return manager.GetMessageWithFallback(keyPath, fallback)
		}
	}
	if globalI18n != nil {
		return TWithFallback(keyPath, fallback)
	}
	// Fallback if i18n is not initialized
	return fallback
}

// Common i18n message keys
const (
	// Success messages
	MsgSuccessDefault   = "response.success.default"
	MsgSuccessCreated   = "response.success.created"
	MsgSuccessUpdated   = "response.success.updated"
	MsgSuccessDeleted   = "response.success.deleted"
	MsgSuccessRetrieved = "response.success.retrieved"

	// Error messages
	MsgErrorValidation         = "response.error.validation"
	MsgErrorNotFound           = "response.error.not_found"
	MsgErrorUnauthorized       = "response.error.unauthorized"
	MsgErrorForbidden          = "response.error.forbidden"
	MsgErrorConflict           = "response.error.conflict"
	MsgErrorInternal           = "response.error.internal"
	MsgErrorBadRequest         = "response.error.bad_request"
	MsgErrorMethodNotAllowed   = "response.error.method_not_allowed"
	MsgErrorRequestTimeout     = "response.error.request_timeout"
	MsgErrorTooManyRequests    = "response.error.too_many_requests"
	MsgErrorServiceUnavailable = "response.error.service_unavailable"
)
