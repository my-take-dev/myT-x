//go:build !windows

package sessionmemo

import "os"

func replaceMemoFile(sourcePath string, targetPath string) error {
	return os.Rename(sourcePath, targetPath)
}
