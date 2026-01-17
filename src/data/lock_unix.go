//go:build unix

package data

import (
	"os"
	"syscall"
)

// LockFile acquires an exclusive lock on path.lock. Blocks until lock is acquired.
func LockFile(path string) (*FileLock, error) {
	lockPath := path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return &FileLock{path: lockPath, file: f}, nil
}

// Unlock releases the lock and removes the lock file.
func (l *FileLock) Unlock() error {
	syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	l.file.Close()
	os.Remove(l.path)
	return nil
}
