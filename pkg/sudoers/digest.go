// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sudoers

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/opencontainers/go-digest"
)

// isPrivilegedHelper reports whether exe is installed in the directory reserved
// for Lima's own privileged helpers: `<prefix>/libexec/lima/privileged`, or
// `<prefix>/lib/lima/privileged` on Homebrew.
func isPrivilegedHelper(exe string) bool {
	dir := filepath.Dir(exe)
	return filepath.Base(dir) == "privileged" && filepath.Base(filepath.Dir(dir)) == "lima"
}

// pinPrivilegedHelpers prefixes every command that runs a privileged helper
// (see isPrivilegedHelper) with the sudoers `Digest_Spec` of that helper.
// sudo (>= 1.8.7) hashes the file immediately before executing it and refuses
// to run it when the digest no longer matches, so an attacker who manages to
// replace a helper cannot get the replacement executed as root. Unlike the
// install-path ownership checks, this one runs inside sudo and is therefore
// free of a time-of-check/time-of-use race.
//
// Pinning is decided by the install location rather than by the caller, so a
// new helper is pinned as soon as it is installed next to the existing ones.
// The consequence is that the sudoers file has to be regenerated whenever a
// pinned helper is reinstalled.
func pinPrivilegedHelpers(commands []string) ([]string, error) {
	digests := make(map[string]digest.Digest)
	pinned := make([]string, 0, len(commands))
	for _, command := range commands {
		exe, _, _ := strings.Cut(command, " ")
		if !isPrivilegedHelper(exe) {
			pinned = append(pinned, command)
			continue
		}
		d, ok := digests[exe]
		if !ok {
			var err error
			if d, err = fileDigest(exe); err != nil {
				return nil, err
			}
			digests[exe] = d
		}
		pinned = append(pinned, d.String()+" "+command)
	}
	return pinned, nil
}

var digestSpecRE = regexp.MustCompile(`sha256:[0-9a-f]{64} `)

// Unpin removes the digests that NOPASSWD adds to a rendered fragment, for
// hosts that opt out of pinning with `limactl sudoers --digest=false`. sudo then
// runs whatever binary is installed at the helper path, so the file does not
// have to be regenerated when the helper is upgraded.
func Unpin(fragment string) string {
	return digestSpecRE.ReplaceAllString(fragment, "")
}

// ContainsFragment is ContainsActiveFragment that also accepts the unpinned
// form of fragment. A pinned grant with a stale digest is still rejected.
func ContainsFragment(file, fragment string) bool {
	return ContainsActiveFragment(file, fragment) || ContainsActiveFragment(file, Unpin(fragment))
}

func fileDigest(binary string) (digest.Digest, error) {
	f, err := os.Open(binary)
	if err != nil {
		return "", err
	}
	defer f.Close()
	d, err := digest.FromReader(f)
	if err != nil {
		return "", fmt.Errorf("failed to digest %#q: %w", binary, err)
	}
	return d, nil
}
