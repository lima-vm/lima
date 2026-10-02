// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

// ReadGuestOSVersion returns the macOS version (e.g. "27.0.0") and build
// version (e.g. "26A428") of the restore image that a macOS guest on the vz
// driver was installed from. Both are empty when unknown: for non-macOS guests,
// and for instances installed before the version was recorded.
func ReadGuestOSVersion(instDir string) (version, buildVersion string) {
	return readOptionalFile(filepath.Join(instDir, filenames.VzGuestOSVersion)),
		readOptionalFile(filepath.Join(instDir, filenames.VzGuestOSBuildVersion))
}

func readOptionalFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logrus.WithError(err).Warnf("Failed to read %#q", path)
		}
		return ""
	}
	return strings.TrimSpace(string(b))
}
