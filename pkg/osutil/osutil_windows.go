// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

// UnixPathMax is the value of UNIX_PATH_MAX.
const UnixPathMax = 108

// Stat is a selection of syscall.Stat_t
type Stat struct {
	Uid uint32
	Gid uint32
}

func SysStat(_ fs.FileInfo) (Stat, bool) {
	return Stat{Uid: 0, Gid: 0}, false
}

// SigInt is the value of SIGINT.
const SigInt = Signal(2)

// SigKill is the value of SIGKILL.
const SigKill = Signal(9)

type Signal int

func SysKill(pid int, _ Signal) error {
	return windows.GenerateConsoleCtrlEvent(syscall.CTRL_BREAK_EVENT, uint32(pid))
}

func Dup2(oldfd int, newfd syscall.Handle) (err error) {
	return fmt.Errorf("unimplemented")
}

func SignalName(sig os.Signal) string {
	switch sig {
	case syscall.SIGINT:
		return "SIGINT"
	case syscall.SIGTERM:
		return "SIGTERM"
	default:
		return fmt.Sprintf("Signal(%d)", sig)
	}
}

func Sysctl(name string) (string, error) {
	return "", errors.New("sysctl: unimplemented on Windows")
}
