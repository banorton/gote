//go:build !darwin

package data

import (
	"os"
	"time"
)

// GetBirthtime returns ModTime on non-macOS platforms (birthtime not available)
func GetBirthtime(info os.FileInfo) time.Time {
	return info.ModTime()
}
