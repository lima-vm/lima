// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package strutil

import "unicode"

// RemoveControlChars drops non-printable runes so guest-controlled text is safe
// to print to the operator's terminal.
func RemoveControlChars(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if unicode.IsPrint(r) {
			out = append(out, r)
		}
	}
	return string(out)
}
