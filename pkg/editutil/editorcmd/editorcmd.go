// From https://raw.githubusercontent.com/norouter/norouter/v0.6.5/cmd/norouter/editorcmd/editorcmd.go
/*
   Copyright (C) NoRouter authors.

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

package editorcmd

import (
	"os"
	"os/exec"
)

// Detect detects a text editor command.
// Returns an empty string when no editor is found.
func Detect() string {
	var candidates = []string{
		os.Getenv("VISUAL"),
		os.Getenv("EDITOR"),
		"editor",
		"vim",
		"vi",
		"emacs",
	}
	for _, f := range candidates {
		if f == "" {
			continue
		}
		x, err := exec.LookPath(f)
		if err == nil {
			return x
		}
	}
	return ""
}
