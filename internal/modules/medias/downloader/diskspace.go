package downloader

import "fmt"

const diskSpaceReserveBytes int64 = 512 * 1024 * 1024

type DiskSpaceError struct {
	Needed int64
	Free   int64
}

func (e *DiskSpaceError) Error() string {
	return fmt.Sprintf("insufficient disk space: needed at least %d bytes, but only %d bytes are free", e.Needed, e.Free)
}

func checkDiskSpace(neededBytes int64) error {
	if neededBytes <= 0 {
		return nil
	}
	free, err := availableTempSpace()
	if err != nil {
		return nil
	}
	if neededBytes+diskSpaceReserveBytes > free {
		return &DiskSpaceError{Needed: neededBytes + diskSpaceReserveBytes, Free: free}
	}
	return nil
}
