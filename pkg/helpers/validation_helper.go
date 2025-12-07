package helpers

import (
	"strings"
)

// IsValidUUID checks if a string is a valid UUID
func IsValidUUID(uuid string) bool {
	if len(uuid) != 36 {
		return false
	}

	// Simple UUID format validation (8-4-4-4-12 format)
	parts := strings.Split(uuid, "-")
	if len(parts) != 5 {
		return false
	}

	if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		return false
	}

	return true
}
