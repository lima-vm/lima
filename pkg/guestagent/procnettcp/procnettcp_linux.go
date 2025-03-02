/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
