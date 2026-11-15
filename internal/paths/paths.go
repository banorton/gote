package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

func ConfigDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "gote")
	}
	return filepath.Join(home(), ".config", "gote")
}

func stateBase() string {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "gote")
	}
	return filepath.Join(home(), ".local", "state", "gote")
}

// StateDir returns the state directory, creating it with mode 0700.
func StateDir() (string, error) {
	d := stateBase()
	if err := os.MkdirAll(d, 0o700); err != nil {
		return "", fmt.Errorf("create state dir %s: %w", d, err)
	}
	return d, nil
}

func DBPath() (string, error) {
	d, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "gote.db"), nil
}

func SocketPath() (string, error) {
	d, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "gote.sock"), nil
}
