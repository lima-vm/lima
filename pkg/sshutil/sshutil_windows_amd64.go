package sshutil

import (
	"github.com/intel-go/cpuid"
)

func detectAESAcceleration() bool {
	return cpuid.HasFeature(cpuid.AES)
}
