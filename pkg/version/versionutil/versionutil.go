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

// GreaterThan returns true if the Lima version used to create an instance is greater
// than a specific older version. Always returns false if the Lima version is the empty string.
// Unparsable lima versions (like SHA1 commit ids) are treated as the latest version and return true.
// limaVersion is a `github describe` string, not a semantic version. So "0.19.1-16-gf3dc6ed.m"
// will be considered greater than "0.19.1".
func GreaterThan(limaVersion, oldVersion string) bool {
	if limaVersion == "" {
		return false
	}
	version, err := Parse(limaVersion)
	if err != nil {
		return true
	}
	switch version.Compare(*semver.New(oldVersion)) {
	case -1:
		return false
	case +1:
		return true
	case 0:
		return strings.Contains(limaVersion, "-")
	}
	panic("unreachable")
}
