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
func ExpandHome(orig string, homeDir string) (string, error) {
	s := orig
	if s == "" {
		return "", errors.New("empty path")
	}

	if strings.HasPrefix(s, "~") {
		if s == "~" || strings.HasPrefix(s, "~/") {
			s = strings.Replace(s, "~", homeDir, 1)
		} else {
			// Paths like "~foo/bar" are unsupported.
			return "", fmt.Errorf("unexpandable path %q", orig)
		}
	}
	return s, nil
}

// Expand expands a path like "~", "~/", "~/foo", on the host.
func Expand(orig string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	s, err := ExpandHome(orig, homeDir)
	if err != nil {
		return "", err
	}
	return filepath.Abs(s)
}
