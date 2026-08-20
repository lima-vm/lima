// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sudoers

import (
	"fmt"
	"os"

	"github.com/opencontainers/go-digest"
)

// Pin prefixes every command with the sudoers `Digest_Spec` of binary, which
// all of them must run. sudo (>= 1.8.7) hashes the file immediately before
// executing it and refuses to run it when the digest no longer matches, so an
// attacker who manages to replace a helper under libexec/lima/privileged cannot
// get the replacement executed as root. Unlike the install-path ownership
// checks, this one runs inside sudo and is therefore free of a
// time-of-check/time-of-use race.
//
// The consequence is that the sudoers file has to be regenerated whenever a
// pinned helper is reinstalled.
func Pin(binary string, commands ...string) ([]string, error) {
	f, err := os.Open(binary)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	d, err := digest.FromReader(f)
	if err != nil {
		return nil, fmt.Errorf("failed to digest %#q: %w", binary, err)
	}
	pinned := make([]string, 0, len(commands))
	for _, command := range commands {
		pinned = append(pinned, d.String()+" "+command)
	}
	return pinned, nil
}
