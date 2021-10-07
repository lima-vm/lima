//go:build !darwin && !linux
// +build !darwin,!linux

package sshutil

import (
	"runtime"

	"github.com/sirupsen/logrus"
)

func detectAESAcceleration() bool {
	const fallback = runtime.GOARCH == "amd64"
	logrus.WithError(err).Warnf("cannot detect whether AES accelerator is available, assuming %v", fallback)
	return fallback
}
