// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sshutil

import (
	"fmt"
	"path/filepath"

	"github.com/lima-vm/lima/pkg/ioutilx"
	"github.com/lima-vm/lima/pkg/store/filenames"
)

func identityFileEntry(privateKeyPath string) (string, error) {
	privateKeyPath, err := ioutilx.WindowsSubsystemPath(privateKeyPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`IdentityFile='%s'`, privateKeyPath), nil
}

func controlPath(controlSock string) (string, error) {
	sock, err := ioutilx.WindowsSubsystemPath(controlSock)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`ControlPath='%s'`, sock), nil
}

func privPath(configDir string) (string, error) {
	privPath, err := ioutilx.WindowsSubsystemPath(filepath.Join(configDir, filenames.UserPrivateKey))
	if err != nil {
		return "", err
	}
	return privPath, nil
}

func sshCiphersOption(algorithm string) string {
	return fmt.Sprintf("Ciphers=%s", algorithm)
}
