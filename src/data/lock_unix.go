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
	// Remove lock file before releasing the flock so other waiters
	// don't see a stale file after we unlock.
	removeErr := os.Remove(l.path)
	if err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN); err != nil {
		l.file.Close()
		return err
	}
	if err := l.file.Close(); err != nil {
		return err
	}
	return removeErr
}
