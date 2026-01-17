package data

import "os"

// FileLock provides exclusive file locking.
type FileLock struct {
	path string
	file *os.File
}
