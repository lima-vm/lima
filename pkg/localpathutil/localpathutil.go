package localpathutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Expand expands a path like "~", "~/", "~/foo".
// Paths like "~foo/bar" are unsupported.
//
// FIXME: is there an existing library for this?
func Expand(orig string) (string, error) {
	s := orig
	if s == "" {
		return "", errors.New("empty path")
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(s, "~") {
		if s == "~" || strings.HasPrefix(s, "~/") {
			s = strings.Replace(s, "~", homeDir, 1)
		} else {
			// Paths like "~foo/bar" are unsupported.
			return "", fmt.Errorf("unexpandable path %q", orig)
		}
	}
	return filepath.Abs(s)
}

// Path converts a filepath to a path
// Unix:        /foo/bar => /foo/bar
// Windows:     C:\foo\bar => /c/foo/bar
func Path(orig string) string {
	s := filepath.ToSlash(orig)
	vol := filepath.VolumeName(orig)
	if vol != "" && strings.Contains(vol, ":") {
		v := "/"
		v += strings.ToLower(vol[0:1])
		s = strings.Replace(s, vol, v, 1)
	}
	return s
}
