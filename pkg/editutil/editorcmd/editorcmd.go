// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

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
	"runtime"
	"strings"
)

// shellChars are the characters that make Detect hand $VISUAL or $EDITOR to the shell,
// like git does. Whitespace alone is not included, so a value like "code -w" is split
// into fields and run directly, which also works on Windows.
const shellChars = "|&;<>()$`\\\"'*?[#~=%"

// Detect detects a text editor command.
// The returned slice is the command and its arguments, to which the caller appends
// the file to edit.
// Returns nil when no editor is found.
func Detect() []string {
	candidates := []string{
		os.Getenv("VISUAL"),
		os.Getenv("EDITOR"),
		"editor",
		"vim",
		"vi",
		"emacs",
	}
	for _, f := range candidates {
		if args := resolve(f); args != nil {
			return args
		}
	}
	return nil
}

// resolve returns the command for the editor string, or nil if it cannot be run.
func resolve(editor string) []string {
	if strings.TrimSpace(editor) == "" {
		return nil
	}
	// A plain executable name or path, which may itself contain whitespace.
	if x, err := exec.LookPath(editor); err == nil {
		return []string{x}
	}
	if runtime.GOOS != "windows" && strings.ContainsAny(editor, shellChars) {
		// Let the shell parse the quoting. "$@" expands to the file to edit.
		return []string{"/bin/sh", "-c", editor + ` "$@"`, editor}
	}
	fields := strings.Fields(editor)
	if len(fields) < 2 {
		return nil
	}
	x, err := exec.LookPath(fields[0])
	if err != nil {
		return nil
	}
	return append([]string{x}, fields[1:]...)
}
