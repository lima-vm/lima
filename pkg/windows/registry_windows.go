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

package windows

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	guestCommunicationsPrefix = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Virtualization\GuestCommunicationServices`
	magicVSOCKSuffix          = "-facb-11e6-bd58-64006a7986d3"
	wslDistroInfoPrefix       = `SOFTWARE\Microsoft\Windows\CurrentVersion\Lxss`
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
		return fmt.Errorf("port %q in use", port)
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

// GetDistroID returns a DistroId GUID corresponding to a Lima instance name.
func GetDistroID(name string) (string, error) {
	rootKey, err := registry.OpenKey(
		registry.CURRENT_USER,
		wslDistroInfoPrefix,
		registry.READ,
	)
	if err != nil {
		return "", fmt.Errorf(
			"failed to open Lxss key (%s): %w",
			wslDistroInfoPrefix,
			err,
		)
	}
	defer rootKey.Close()

	keys, err := rootKey.ReadSubKeyNames(-1)
	if err != nil {
		return "", fmt.Errorf("failed to read subkey names for %s: %w", wslDistroInfoPrefix, err)
	}

	var out string
	for _, k := range keys {
		subKey, err := registry.OpenKey(
			registry.CURRENT_USER,
			fmt.Sprintf(`%s\%s`, wslDistroInfoPrefix, k),
			registry.READ,
		)
		if err != nil {
			return "", fmt.Errorf("failed to read subkey %q for key %q: %w", k, wslDistroInfoPrefix, err)
		}
		dn, _, err := subKey.GetStringValue("DistributionName")
		if err != nil {
			return "", fmt.Errorf("failed to read 'DistributionName' value for subkey %q of %q: %w", k, wslDistroInfoPrefix, err)
		}
		if dn == name {
			out = k
			break
		}
	}

	if out == "" {
		return "", fmt.Errorf("failed to find matching DistroID for %q", name)
	}

	return out, nil
}

// GetRandomFreeVSockPort gets a list of all registered VSock ports and returns a non-registered port.
func GetRandomFreeVSockPort(min, max int) (int, error) {
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
	tree[0] = pair{0, min}

	sort.Ints(used)
	for i, v := range used {
		if tree[len(tree)-1].v+tree[len(tree)-1].offset == v {
			tree[len(tree)-1].offset++
		} else {
			tree = append(tree, pair{v - min - i, min + i + 1})
		}
	}

	v := rand.IntN(max - min + 1 - len(used))

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
				return nil, fmt.Errorf("failed convert %q to int: %w", split[0], err)
			}
			out = append(out, i)
		}
	}

	return out, nil
}
