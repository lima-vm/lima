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
