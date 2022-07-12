//go:build !windows
// +build !windows

package dirnames

// ShortPathName just returns the provided path.
func ShortPathName(path string) (string, error) {
	return path, nil
}
