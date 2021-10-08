package sshutil

import (
	"fmt"
	"runtime"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"
)

func detectAESAcceleration() bool {
	switch runtime.GOARCH {
	case "amd64":
		const fallback = true
		features, err := syscall.Sysctl("machdep.cpu.features") // not available on M1
		if err != nil {
			err = fmt.Errorf("failed to read sysctl \"machdep.cpu.features\": %w", err)
			logrus.WithError(err).Warnf("failed to detect whether AES accelerator is available, assuming %v", fallback)
			return fallback
		}
		return strings.Contains(features, "AES ")
	default:
		// According to https://gist.github.com/voluntas/fd279c7b4e71f9950cfd4a5ab90b722b ,
		// aes-128-gcm is faster than chacha20-poly1305 on Apple M1.
		return true
	}
}
