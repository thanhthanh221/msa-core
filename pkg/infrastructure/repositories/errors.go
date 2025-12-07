package repositories

import (
	"gorm.io/gorm"
)

var (
	ErrNotFound = gorm.ErrRecordNotFound
)
