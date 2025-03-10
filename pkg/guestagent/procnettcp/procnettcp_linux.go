// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package procnettcp

import (
	"errors"
	"os"
)

// ParseFiles parses /proc/net/{tcp, tcp6}.
func ParseFiles() ([]Entry, error) {
	var res []Entry
	files := map[string]Kind{
		"/proc/net/tcp":  TCP,
		"/proc/net/tcp6": TCP6,
		"/proc/net/udp":  UDP,
		"/proc/net/udp6": UDP6,
	}
	for file, kind := range files {
		r, err := os.Open(file)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return res, err
		}
		parsed, err := Parse(r, kind)
		if err != nil {
			_ = r.Close()
			return res, err
		}
		_ = r.Close()
		res = append(res, parsed...)
	}
	return res, nil
}
