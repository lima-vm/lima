// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package windows

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	guestCommunicationsPrefix = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Virtualization\GuestCommunicationServices`
	magicVSOCKSuffix          = "-facb-11e6-bd58-64006a7986d3"
	wslInfoKeyPath            = `SOFTWARE\Microsoft\Windows\CurrentVersion\Lxss`
)

// AddVSockRegistryKey makes a vsock server running on the host accessible in guests.
func AddVSockRegistryKey(port int) error {
	rootKey, err := getGuestCommunicationServicesKey(true)
	if err != nil {
		return err
	}
	defer rootKey.Close()

	used, err := getUsedPorts(rootKey)
	if err != nil {
		return err
	}

	if slices.Contains(used, port) {
		return fmt.Errorf("port %d in use", port)
	}

	vsockKeyPath := fmt.Sprintf(`%x%s`, port, magicVSOCKSuffix)
	vSockKey, _, err := registry.CreateKey(
		rootKey,
		vsockKeyPath,
		registry.ALL_ACCESS,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to create new key (%s%s): %w",
			guestCommunicationsPrefix,
			vsockKeyPath,
			err,
		)
	}
	defer vSockKey.Close()

	return nil
}

// RemoveVSockRegistryKey removes entries created by AddVSockRegistryKey.
func RemoveVSockRegistryKey(port int) error {
	rootKey, err := getGuestCommunicationServicesKey(true)
	if err != nil {
		return err
	}
	defer rootKey.Close()

	vsockKeyPath := fmt.Sprintf(`%x%s`, port, magicVSOCKSuffix)
	if err := registry.DeleteKey(rootKey, vsockKeyPath); err != nil {
		return fmt.Errorf(
			"failed to create new key (%s%s): %w",
			guestCommunicationsPrefix,
			vsockKeyPath,
			err,
		)
	}

	return nil
}

// IsVSockPortFree determines if a VSock port has been registered already.
func IsVSockPortFree(port int) (bool, error) {
	rootKey, err := getGuestCommunicationServicesKey(false)
	if err != nil {
		return false, err
	}
	defer rootKey.Close()

	used, err := getUsedPorts(rootKey)
	if err != nil {
		return false, err
	}

	if slices.Contains(used, port) {
		return false, nil
	}

	return true, nil
}

// IsWSLInstalled checks whether WSL is (probably) installed.
func IsWSLInstalled() (bool, error) {
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		wslInfoKeyPath,
		registry.QUERY_VALUE,
	)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("failed to open WSL registry key: %w", err)
	}
	defer key.Close()

	keyInfo, err := key.Stat()
	if err != nil {
		return false, fmt.Errorf("failed to stat WSL registry key: %w", err)
	}

	// The key should exist even if WSL is not installed; however, installing
	// WSL creates subkeys ("MSI", and "Plugins"), so assume WSL is installed if
	// there are any subkeys.
	return keyInfo.SubKeyCount > 0, nil
}

// GetDistroID returns a DistroId GUID corresponding to a Lima instance name.
func GetDistroID(name string) (string, error) {
	rootKey, err := registry.OpenKey(
		registry.CURRENT_USER,
		wslInfoKeyPath,
		registry.READ,
	)
	if err != nil {
		return "", fmt.Errorf(
			"failed to open Lxss key (%s): %w",
			wslInfoKeyPath,
			err,
		)
	}
	defer rootKey.Close()

	keys, err := rootKey.ReadSubKeyNames(-1)
	if err != nil {
		return "", fmt.Errorf("failed to read subkey names for %s: %w", wslInfoKeyPath, err)
	}

	var out string
	for _, k := range keys {
		subKey, err := registry.OpenKey(rootKey, k, registry.READ)
		if err != nil {
			return "", fmt.Errorf("failed to read subkey %#q for key %#q: %w", k, wslInfoKeyPath, err)
		}
		dn, _, err := subKey.GetStringValue("DistributionName")
		subKey.Close()
		if err != nil {
			return "", fmt.Errorf("failed to read 'DistributionName' value for subkey %#q of %#q: %w", k, wslInfoKeyPath, err)
		}
		if dn == name {
			out = k
			break
		}
	}

	if out == "" {
		return "", fmt.Errorf("failed to find matching DistroID for %#q", name)
	}

	return out, nil
}

// GetRandomFreeVSockPort gets a list of all registered VSock ports and returns a non-registered port.
func GetRandomFreeVSockPort(minPort, maxPort int) (int, error) {
	rootKey, err := getGuestCommunicationServicesKey(false)
	if err != nil {
		return 0, err
	}
	defer rootKey.Close()

	used, err := getUsedPorts(rootKey)
	if err != nil {
		return 0, err
	}

	type pair struct{ v, offset int }
	tree := make([]pair, 1, len(used)+1)
	tree[0] = pair{0, minPort}

	slices.Sort(used)
	for i, v := range used {
		if tree[len(tree)-1].v+tree[len(tree)-1].offset == v {
			tree[len(tree)-1].offset++
		} else {
			tree = append(tree, pair{v - minPort - i, minPort + i + 1})
		}
	}

	v := rand.IntN(maxPort - minPort + 1 - len(used))

	for len(tree) > 1 {
		m := len(tree) / 2
		if v < tree[m].v {
			tree = tree[:m]
		} else {
			tree = tree[m:]
		}
	}

	return tree[0].offset + v, nil
}

// getGuestCommunicationServicesKey returns the HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Virtualization\GuestCommunicationServices
// registry key for use in other operations.
//
// allowWrite is configurable because setting it to true requires Administrator access.
func getGuestCommunicationServicesKey(allowWrite bool) (registry.Key, error) {
	var registryPermissions uint32 = registry.READ
	if allowWrite {
		registryPermissions = registry.WRITE | registry.READ
	}
	rootKey, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		guestCommunicationsPrefix,
		registryPermissions,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to open GuestCommunicationServices key (%s): %w",
			guestCommunicationsPrefix,
			err,
		)
	}

	return rootKey, nil
}

func getUsedPorts(key registry.Key) ([]int, error) {
	keys, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return nil, fmt.Errorf("failed to read subkey names for %s: %w", guestCommunicationsPrefix, err)
	}

	out := []int{}
	for _, k := range keys {
		split := strings.Split(k, magicVSOCKSuffix)
		if len(split) == 2 {
			i, err := strconv.Atoi(split[0])
			if err != nil {
				return nil, fmt.Errorf("failed convert %#q to int: %w", split[0], err)
			}
			out = append(out, i)
		}
	}

	return out, nil
}
