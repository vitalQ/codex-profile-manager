//go:build !windows

package fsx

import "os"

func ReplaceFile(sourcePath, targetPath string) error {
	return os.Rename(sourcePath, targetPath)
}
