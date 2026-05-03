//go:build windows

package sessionmemo

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func replaceMemoFile(sourcePath string, targetPath string) error {
	source, err := windows.UTF16PtrFromString(sourcePath)
	if err != nil {
		return fmt.Errorf("convert source path: %w", err)
	}
	target, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		return fmt.Errorf("convert target path: %w", err)
	}
	if err := windows.MoveFileEx(
		source,
		target,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	); err != nil {
		return err
	}
	return nil
}
