package freeport

import "github.com/lima-vm/lima/pkg/windows"

func VSock() (int, error) {
	return windows.GetRandomFreeVSockPort(0, 2147483647)
}
