//go:build !windows

package downloader

import (
	"os"
	"syscall"
)

func availableTempSpace() (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(os.TempDir(), &stat); err != nil {
		return 0, err
	}
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}
