// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package fsutil

import (
	"fmt"
	"strings"
)

// TranslateWindowsToWSLPath translates a Windows path (e.g., C:\Users\...) to a WSL-compliant path (e.g., /mnt/c/Users/...).
func TranslateWindowsToWSLPath(orig string) (string, error) {
	cleaned := orig
	// Trim trailing slashes/backslashes, but keep drive root if length <= 2
	for len(cleaned) > 2 && (cleaned[len(cleaned)-1] == '/' || cleaned[len(cleaned)-1] == '\\') {
		cleaned = cleaned[:len(cleaned)-1]
	}

	// UNC paths check
	if strings.HasPrefix(cleaned, `\\`) || strings.HasPrefix(cleaned, `//`) {
		return "", fmt.Errorf("UNC paths are not supported for WSL translation: %q", orig)
	}

	// Recognizable drive letter paths like C: or C:\path or C:/path
	if len(cleaned) >= 2 && cleaned[1] == ':' && ((cleaned[0] >= 'A' && cleaned[0] <= 'Z') || (cleaned[0] >= 'a' && cleaned[0] <= 'z')) {
		drive := strings.ToLower(string(cleaned[0]))
		if len(cleaned) == 2 {
			return "/mnt/" + drive, nil
		}
		// Must be followed by a slash or backslash
		if cleaned[2] == '/' || cleaned[2] == '\\' {
			rest := strings.ReplaceAll(cleaned[2:], "\\", "/")
			return "/mnt/" + drive + rest, nil
		}
	}

	return "", fmt.Errorf("not an absolute Windows path with drive letter: %q", orig)
}
