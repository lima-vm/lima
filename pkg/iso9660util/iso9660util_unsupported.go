//+build !darwin

package iso9660util

import (
	"runtime"

	"github.com/pkg/errors"
)

func Write(isoFilePath string, iso *ISO9660) error {
	return errors.Errorf("unsupported GOOS: %q", runtime.GOOS)
}
