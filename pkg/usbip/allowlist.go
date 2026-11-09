// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lima-vm/lima/v2/pkg/fsutil"
	"github.com/lima-vm/lima/v2/pkg/lockutil"
)

const allowlistFile = "usbip-allow.json"

type AllowEntry struct {
	Name      string `json:"name,omitempty"`
	VendorID  string `json:"vendorID"`
	ProductID string `json:"productID"`
	BusAddr   string `json:"busAddr,omitempty"`
}

func ReadAllowlist(instDir string) ([]AllowEntry, error) {
	b, err := os.ReadFile(filepath.Join(instDir, allowlistFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []AllowEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, fmt.Errorf("parsing usbip allowlist: %w", err)
	}
	return entries, nil
}

func WriteAllowlist(instDir string, entries []AllowEntry) error {
	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(filepath.Join(instDir, allowlistFile), b)
}

// UpdateAllowlist locks, since several processes write the file.
func UpdateAllowlist(instDir string, fn func([]AllowEntry) []AllowEntry) error {
	return lockutil.WithDirLock(instDir, func() error {
		list, err := ReadAllowlist(instDir)
		if err != nil {
			return err
		}
		return WriteAllowlist(instDir, fn(list))
	})
}
