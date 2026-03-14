// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// From https://github.com/containerd/containerd/blob/v2.1.1/pkg/identifiers/validate.go
// SPDX-FileCopyrightText: Copyright The containerd Authors
// LICENSE: https://github.com/containerd/containerd/blob/v2.1.1/LICENSE
// NOTICE: https://github.com/containerd/containerd/blob/v2.1.1/NOTICE

/*
   Copyright The containerd Authors.

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

// Package identifiers provides common validation for identifiers and keys
// across Lima (originally from containerd).
//
// Identifiers in Lima must be a alphanumeric, allowing limited
// underscores, dashes and dots.
//
// While the character set may be expanded in the future, identifiers
// are guaranteed to be safely used as filesystem path components.
package identifiers

import (
	"errors"
	"fmt"
	"regexp"
)

const (
	maxLength  = 76
	alphanum   = `[A-Za-z0-9]+`
	separators = `[._-]`
)

// identifierRe defines the pattern for valid identifiers.
var identifierRe = regexp.MustCompile(reAnchor(alphanum + reGroup(separators+reGroup(alphanum)) + "*"))

// Validate returns nil if the string s is a valid identifier.
//
// Identifiers are similar to the domain name rules according to RFC 1035, section 2.3.1. However
// rules in this package are relaxed to allow numerals to follow period (".") and mixed case is
// allowed.
//
// In general identifiers that pass this validation should be safe for use as filesystem path components.
func Validate(s string) error {
	if s == "" {
		return errors.New("identifier must not be empty")
	}

	if len(s) > maxLength {
		return fmt.Errorf("identifier %q greater than maximum length (%d characters)", s, maxLength)
	}

	if !identifierRe.MatchString(s) {
		return fmt.Errorf("identifier %q must match %v", s, identifierRe)
	}
	return nil
}

func reGroup(s string) string {
	return `(?:` + s + `)`
}

func reAnchor(s string) string {
	return `^` + s + `$`
}
