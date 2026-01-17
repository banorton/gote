//go:build windows

package data

import "os"

// LockFile on Windows returns a no-op lock.
// Atomic writes still protect against corruption.
func LockFile(path string) (*FileLock, error) {
	lockPath := path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	return &FileLock{path: lockPath, file: f}, nil
}

// Unlock releases the lock file.
func (l *FileLock) Unlock() error {
	l.file.Close()
	os.Remove(l.path)
	return nil
}
