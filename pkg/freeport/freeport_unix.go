//go:build !windows

package freeport

import "errors"

func VSock() (int, error) {
	return 0, errors.New("freeport.VSock is not implemented for non-Windows hosts")
}
