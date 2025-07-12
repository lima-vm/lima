//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sshutil

import (
	"fmt"
	"path/filepath"

	"github.com/lima-vm/lima/pkg/store/filenames"
)

func identityFileEntry(privateKeyPath string) (string, error) {
	return fmt.Sprintf(`IdentityFile="%s"`, privateKeyPath), nil
}

func controlPath(controlSock string) (string, error) {
	return fmt.Sprintf(`ControlPath="%s"`, controlSock), nil
}

func privPath(configDir string) (string, error) {
	return filepath.Join(configDir, filenames.UserPrivateKey), nil
}

func sshCiphersOption(algorithm string) string {
	return fmt.Sprintf("Ciphers=%q", algorithm)
}
