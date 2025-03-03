// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package windows

import (
	"fmt"
	"regexp"
	"strings"
)

// vmIDRegex is a regular expression to extract the VM ID from the command line of wslhost.exe.
var vmIDRegex = regexp.MustCompile(`--vm-id\s\{(?P<vmID>.{36})\}`)

// GetInstanceVMID returns the VM ID of a running WSL instance.
func GetInstanceVMID(instanceName string) (string, error) {
	distroID, err := GetDistroID(instanceName)
	if err != nil {
		return "", err
	}

	cmdLines, err := GetProcessCommandLine("wslhost.exe")
	if err != nil {
		return "", err
	}

	vmID := ""
	for _, cmdLine := range cmdLines {
		if strings.Contains(cmdLine, distroID) {
			if matches := vmIDRegex.FindStringSubmatch(cmdLine); matches != nil {
				vmID = matches[vmIDRegex.SubexpIndex("vmID")]
				break
			}
		}
	}

	if vmID == "" {
		return "", fmt.Errorf("failed to find VM ID for instance %q", instanceName)
	}

	return vmID, nil
}
