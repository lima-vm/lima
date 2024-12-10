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
