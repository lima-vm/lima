package procnettcp

import (
	"errors"
	"os"
)

// ParseFiles parses /proc/net/{tcp, tcp6}
func ParseFiles() ([]Entry, error) {
	var res []Entry
	files := map[string]Kind{
		"/proc/net/tcp":  TCP,
		"/proc/net/tcp6": TCP6,
	}
	for file, kind := range files {
		r, err := os.Open(file)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return res, err
		}
		defer r.Close()
		parsed, err := Parse(r, kind)
		if err != nil {
			return res, err
		}
		res = append(res, parsed...)
	}
	return res, nil
}
