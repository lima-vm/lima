// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lima-vm/lima/v2/pkg/fsutil"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/lockutil"
)

const allowlistFile = "usbip-allow.json"

type AllowEntry struct {
	Name      string `json:"name,omitempty"`
	VendorID  string `json:"vendorID"`
	ProductID string `json:"productID"`
	BusAddr   string `json:"busAddr,omitempty"`
	Busid     string `json:"busid,omitempty"`
	// RegistryID ends a runtime grant at unplug even with no session to notice it.
	RegistryID uint64 `json:"registryID,omitempty"`
	// Denied overrides a wildcard for one device, after a detach.
	Denied bool `json:"denied,omitempty"`
}

// EntryFor sets Busid, which only runtime grants have.
func EntryFor(info DeviceInfo) AllowEntry {
	return AllowEntry{
		VendorID:   fmt.Sprintf("%04x", info.Vendor),
		ProductID:  fmt.Sprintf("%04x", info.Product),
		BusAddr:    info.Busid,
		Busid:      info.Busid,
		RegistryID: info.RegistryID,
	}
}

func ConfigEntries(devs []limatype.USBDevice) []AllowEntry {
	list := []AllowEntry{}
	// AddEntry keeps the last duplicate, and a template's entry precedes its base's.
	for _, d := range slices.Backward(devs) {
		list = AddEntry(list, AllowEntry{Name: d.Name, VendorID: fmt.Sprint(d.VendorID), ProductID: fmt.Sprint(d.ProductID), BusAddr: d.BusAddr})
	}
	slices.Reverse(list)
	return list
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

// UpdateAllowlist locks, since limactl and the hostagent both write the file.
func UpdateAllowlist(instDir string, fn func([]AllowEntry) []AllowEntry) error {
	return lockutil.WithDirLock(instDir, func() error {
		list, err := ReadAllowlist(instDir)
		if err != nil {
			return err
		}
		return WriteAllowlist(instDir, fn(list))
	})
}

// An empty BusAddr is a wildcard, so only Allowed may use this: list edits would overwrite entries.
func sameDevice(a, b AllowEntry) bool {
	if !strings.EqualFold(a.VendorID, b.VendorID) || !strings.EqualFold(a.ProductID, b.ProductID) {
		return false
	}
	if a.RegistryID != 0 && b.RegistryID != 0 && a.RegistryID != b.RegistryID {
		return false
	}
	if a.BusAddr != "" && b.BusAddr != "" {
		return a.BusAddr == b.BusAddr
	}
	return true
}

func Allowed(list []AllowEntry, dev AllowEntry) bool {
	allowed := false
	for _, e := range list {
		if sameDevice(e, dev) {
			if e.Denied {
				return false
			}
			allowed = true
		}
	}
	return allowed
}

func AddEntry(list []AllowEntry, entry AllowEntry) []AllowEntry {
	return append(RemoveEntry(list, entry), entry)
}

func RemoveEntry(list []AllowEntry, dev AllowEntry) []AllowEntry {
	return slices.DeleteFunc(list, func(e AllowEntry) bool {
		return strings.EqualFold(e.VendorID, dev.VendorID) && strings.EqualFold(e.ProductID, dev.ProductID) && e.BusAddr == dev.BusAddr
	})
}
