//go:build windows

package downloader

import "errors"

func availableTempSpace() (int64, error) {
	return 0, errors.New("disk space check is not supported on windows")
}
