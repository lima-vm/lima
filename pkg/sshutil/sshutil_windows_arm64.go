package sshutil

import (
	"github.com/sirupsen/logrus"
)

func detectAESAcceleration() bool {
	const fallback = false
	logrus.Warnf("cannot detect whether AES accelerator is available, assuming %v", fallback)
	return fallback
}
