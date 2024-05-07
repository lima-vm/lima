//go:build !linux

package fsutil

func IsNFS(path string) (bool, error) {
	return false, nil
}
