package vz

import (
	"os"
)

// CreateDiskImage is creating disk image with specified filename and filesize.
// For example, if you want to create disk with 64GiB, you can set "64 * 1024 * 1024 * 1024" to size.
//
// Note that if you have specified a pathname which already exists, this function
// returns os.ErrExist error. So you can handle it with os.IsExist function.
func CreateDiskImage(pathname string, size int64) error {
	f, err := os.OpenFile(pathname, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := f.Truncate(size); err != nil {
		return err
	}
	return nil
}
