package sshutil

import (
	"os"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

func detectAESAcceleration() bool {
	const fallback = runtime.GOARCH == "amd64"
	cpuinfo, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		logrus.WithError(err).Warnf("failed to detect whether AES accelerator is available, assuming %v", fallback)
		return fallback
	}
	// Checking "aes " should be enough for x86_64 and aarch64
	return strings.Contains(string(cpuinfo), "aes ")
}
