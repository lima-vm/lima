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

// Package labels provides common validation for labels.
// Labels are similar to [github.com/lima-vm/lima/pkg/identifiers], but allows '/'.
package labels

import (
	"errors"
	"fmt"
	"regexp"
)

const (
	maxLength  = 76
	alphanum   = `[A-Za-z0-9]+`
	separators = `[/._-]` // contains slash, unlike identifiers
)

// labelRe defines the pattern for valid identifiers.
var labelRe = regexp.MustCompile(reAnchor(alphanum + reGroup(separators+reGroup(alphanum)) + "*"))

// Validate returns nil if the string s is a valid label.
//
// Labels are similar to [github.com/lima-vm/lima/pkg/identifiers], but allows '/'.
//
// Labels that pass this validation are NOT safe for use as filesystem path components.
func Validate(s string) error {
	if s == "" {
		return errors.New("label must not be empty")
	}

	if len(s) > maxLength {
		return fmt.Errorf("label %q greater than maximum length (%d characters)", s, maxLength)
	}

	if !labelRe.MatchString(s) {
		return fmt.Errorf("label %q must match %v", s, labelRe)
	}
	return nil
}

func reGroup(s string) string {
	return `(?:` + s + `)`
}

func reAnchor(s string) string {
	return `^` + s + `$`
}
