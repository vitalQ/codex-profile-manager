//go:build windows

package fsx

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func ReplaceFile(sourcePath, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
		return err
	}
	return windows.Rename(sourcePath, targetPath)
}
