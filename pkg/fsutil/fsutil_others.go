//go:build !linux

package fsutil

func IsNFS(string) (bool, error) {
	return false, nil
}
