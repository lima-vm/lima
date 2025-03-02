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

package versionutil

import (
	"strings"

	"github.com/coreos/go-semver/semver"
)

// Parse parses a Lima version string by removing the leading "v" character and
// stripping everything from the first "-" forward (which are `git describe` artifacts and
// not semver pre-release markers). So "v0.19.1-16-gf3dc6ed.m" will be parsed as "0.19.1".
func Parse(version string) (*semver.Version, error) {
	version = strings.TrimPrefix(version, "v")
	version, _, _ = strings.Cut(version, "-")
	return semver.NewVersion(version)
}

func compare(limaVersion, oldVersion string) int {
	if limaVersion == "" {
		if oldVersion == "" {
			return 0
		}
		return -1
	}
	version, err := Parse(limaVersion)
	if err != nil {
		return 1
	}
	cmp := version.Compare(*semver.New(oldVersion))
	if cmp == 0 && strings.Contains(limaVersion, "-") {
		cmp = 1
	}
	return cmp
}

// GreaterThan returns true if the Lima version used to create an instance is greater
// than a specific older version. Always returns false if the Lima version is the empty string.
// Unparsable lima versions (like SHA1 commit ids) are treated as the latest version and return true.
// limaVersion is a `github describe` string, not a semantic version. So "0.19.1-16-gf3dc6ed.m"
// will be considered greater than "0.19.1".
func GreaterThan(limaVersion, oldVersion string) bool {
	return compare(limaVersion, oldVersion) > 0
}

// GreaterEqual return true if limaVersion >= oldVersion.
func GreaterEqual(limaVersion, oldVersion string) bool {
	return compare(limaVersion, oldVersion) >= 0
}
