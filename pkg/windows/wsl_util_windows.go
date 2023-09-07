//go:build windows
// +build windows

package windows

import (
	"fmt"
	"regexp"
	"strings"
)

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

	re := regexp.MustCompile(`--vm-id\s\{(?P<vmID>.{36})\}`)
	if err != nil {
		return "", err
	}

	vmID := ""
	for _, cmdLine := range cmdLines {
		if strings.Contains(cmdLine, distroID) {
			if matches := re.FindStringSubmatch(cmdLine); matches != nil {
				vmID = matches[re.SubexpIndex("vmID")]
				break
			}
		}
	}

	if vmID == "" {
		return "", fmt.Errorf("failed to find VM ID for instance %q", instanceName)
	}

	return vmID, nil
}
