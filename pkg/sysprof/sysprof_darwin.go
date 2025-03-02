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

package sysprof

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

var NetworkData = sync.OnceValues(func() ([]NetworkDataType, error) {
	b, err := SystemProfiler("SPNetworkDataType")
	if err != nil {
		return nil, err
	}
	var networkData SPNetworkDataType
	if err := json.Unmarshal(b, &networkData); err != nil {
		return nil, err
	}
	return networkData.SPNetworkDataType, nil
})

func SystemProfiler(dataType string) ([]byte, error) {
	exe, err := exec.LookPath("system_profiler")
	if err != nil {
		// $PATH may lack /usr/sbin
		exe = "/usr/sbin/system_profiler"
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(exe, dataType, "-json")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.HasPrefix(stderr.String(), "Usage: system_profiler") {
			logrus.Warn("Can't fetch system_profiler data; maybe OS is older than macOS Catalina 10.15")
			return []byte("{}"), nil
		}
		return nil, fmt.Errorf("failed to run %v: stdout=%q, stderr=%q: %w",
			cmd.Args, stdout.String(), stderr.String(), err)
	}
	return stdout.Bytes(), nil
}
