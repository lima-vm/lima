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

package osutil

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

var MachineID = sync.OnceValue(func() string {
	x, err := machineID()
	if err == nil && x != "" {
		return x
	}
	logrus.WithError(err).Debug("failed to get machine ID, falling back to use hostname instead")
	hostname, err := os.Hostname()
	if err != nil {
		panic(fmt.Errorf("failed to get hostname: %w", err))
	}
	return hostname
})

func machineID() (string, error) {
	if runtime.GOOS == "darwin" {
		ioPlatformExpertDeviceCmd := exec.Command("/usr/sbin/ioreg", "-a", "-d2", "-c", "IOPlatformExpertDevice")
		ioPlatformExpertDevice, err := ioPlatformExpertDeviceCmd.CombinedOutput()
		if err != nil {
			return "", err
		}
		return parseIOPlatformUUIDFromIOPlatformExpertDevice(bytes.NewReader(ioPlatformExpertDevice))
	}

	candidates := []string{
		"/etc/machine-id",
		"/var/lib/dbus/machine-id",
		// We don't use "/sys/class/dmi/id/product_uuid"
	}
	for _, f := range candidates {
		b, err := os.ReadFile(f)
		if err == nil {
			return strings.TrimSpace(string(b)), nil
		}
	}
	return "", fmt.Errorf("no machine-id found, tried %v", candidates)
}

func parseIOPlatformUUIDFromIOPlatformExpertDevice(r io.Reader) (string, error) {
	d := xml.NewDecoder(r)
	var (
		elem            string
		elemKeyCharData string
	)
	for {
		tok, err := d.Token()
		if err != nil {
			return "", err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			elem = v.Name.Local
		case xml.EndElement:
			elem = ""
			if v.Name.Local != "key" {
				elemKeyCharData = ""
			}
		case xml.CharData:
			if elem == "string" && elemKeyCharData == "IOPlatformUUID" {
				return string(v), nil
			}
			if elem == "key" {
				elemKeyCharData = string(v)
			}
		}
	}
}
