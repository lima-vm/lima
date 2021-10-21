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

var (
	networkDataOnce   sync.Once
	networkDataCached SPNetworkDataType
	networkDataError  error
)

func NetworkData() ([]NetworkDataType, error) {
	networkDataOnce.Do(func() {
		var jsonBytes []byte
		jsonBytes, networkDataError = SystemProfiler("SPNetworkDataType")
		if networkDataError == nil {
			networkDataError = json.Unmarshal(jsonBytes, &networkDataCached)
		}
	})
	return networkDataCached.SPNetworkDataType, networkDataError
}

func SystemProfiler(dataType string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("system_profiler", dataType, "-json")
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
