package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// I18nManager manages internationalization
type I18nManager struct {
	messages map[string]any
	locale   string
}

// NewI18nManager creates a new I18nManager instance
func NewI18nManager(locale string) (*I18nManager, error) {
	manager := &I18nManager{
		messages: make(map[string]any),
		locale:   locale,
	}

	// Load messages for the specified locale
	if err := manager.loadMessages(locale); err != nil {
		return nil, fmt.Errorf("failed to load messages for locale %s: %w", locale, err)
	}

	return manager, nil
}

// loadMessages loads messages from JSON file
func (i *I18nManager) loadMessages(locale string) error {
	// Try multiple paths to find i18n files
	possiblePaths := []string{
		// Path in Docker container (absolute)
		"/src/i18n",
		// Path relative to working directory
		"src/i18n",
	}

	// Try to get path from runtime caller (development mode)
	_, filename, _, ok := runtime.Caller(1)
	if ok {
		i18nDir := filepath.Join(filepath.Dir(filename), "..", "..", "i18n")
		// Resolve the path to absolute
		if absPath, err := filepath.Abs(i18nDir); err == nil {
			possiblePaths = append([]string{absPath}, possiblePaths...)
		}
	}

	// Check environment variable
	if envPath := os.Getenv("I18N_DIR"); envPath != "" {
		possiblePaths = append([]string{envPath}, possiblePaths...)
	}

	var filePath string
	var lastErr error

	for _, i18nDir := range possiblePaths {
		if i18nDir == "" {
			continue
		}
		filePath = filepath.Join(i18nDir, fmt.Sprintf("%s.json", locale))

		// Check if file exists
		if _, err := os.Stat(filePath); err == nil {
			// File found, read it
			data, err := os.ReadFile(filePath)
			if err != nil {
				lastErr = fmt.Errorf("failed to read i18n file %s: %w", filePath, err)
				continue
			}

			// Parse JSON
			var messages map[string]any
			if err := json.Unmarshal(data, &messages); err != nil {
				lastErr = fmt.Errorf("failed to parse i18n file %s: %w", filePath, err)
				continue
			}

			i.messages = messages
			return nil
		}
		lastErr = fmt.Errorf("file not found: %s", filePath)
	}

	// If no file found, return error with all tried paths
	return fmt.Errorf("failed to find i18n file for locale %s. Tried paths: %v. Last error: %v", locale, possiblePaths, lastErr)
}

// GetMessage retrieves a message by key path (e.g., "response.success.default")
func (i *I18nManager) GetMessage(keyPath string) string {
	keys := strings.Split(keyPath, ".")
	if len(keys) == 0 {
		return keyPath
	}

	// Navigate through the nested structure
	current := i.messages
	for idx, key := range keys {
		if idx == len(keys)-1 {
			// Last key, return the value
			if value, ok := current[key]; ok {
				if str, ok := value.(string); ok {
					return str
				}
			}
			return keyPath
		}

		// Navigate deeper
		if next, ok := current[key]; ok {
			if nextMap, ok := next.(map[string]any); ok {
				current = nextMap
			} else {
				return keyPath
			}
		} else {
			return keyPath
		}
	}

	return keyPath
}

// GetMessageWithFallback retrieves a message with fallback to default locale
func (i *I18nManager) GetMessageWithFallback(keyPath string, fallback string) string {
	message := i.GetMessage(keyPath)
	if message == keyPath {
		return fallback
	}
	return message
}

// SetLocale changes the current locale and reloads messages
func (i *I18nManager) SetLocale(locale string) error {
	i.locale = locale
	return i.loadMessages(locale)
}

// GetLocale returns the current locale
func (i *I18nManager) GetLocale() string {
	return i.locale
}

// Global i18n manager instance
var globalI18n *I18nManager

// InitGlobalI18n initializes the global i18n manager
func InitGlobalI18n(locale string) error {
	var err error
	globalI18n, err = NewI18nManager(locale)
	return err
}

// GetGlobalI18n returns the global i18n manager
// It always returns a non-nil manager, creating an empty one if initialization fails
func GetGlobalI18n() *I18nManager {
	if globalI18n == nil {
		// Try to initialize with default locale
		if err := InitGlobalI18n("en"); err != nil {
			// If initialization fails, create an empty manager to prevent nil pointer
			globalI18n = &I18nManager{
				messages: make(map[string]any),
				locale:   "en",
			}
		}
	}
	return globalI18n
}

// T is a shorthand for getting a message from global i18n manager
func T(keyPath string) string {
	manager := GetGlobalI18n()
	if manager == nil {
		return keyPath
	}
	return manager.GetMessage(keyPath)
}

// TWithFallback is a shorthand for getting a message with fallback from global i18n manager
func TWithFallback(keyPath string, fallback string) string {
	manager := GetGlobalI18n()
	if manager == nil {
		return fallback
	}
	return manager.GetMessageWithFallback(keyPath, fallback)
}
