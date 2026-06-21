//go:build windows

package config

import (
	"errors"
	"os"
)

func replaceFile(from, to string) error {
	if err := os.Remove(to); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(from, to)
}
