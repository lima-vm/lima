package osutil

import (
	"io/fs"
	"syscall"
)

// UnixPathMax is the value of UNIX_PATH_MAX.
const UnixPathMax = 108

// Stat is a selection of syscall.Stat_t.
type Stat struct {
	Uid uint32
	Gid uint32
}

func SysStat(fi fs.FileInfo) (Stat, bool) {
	stat, ok := fi.Sys().(*syscall.Stat_t)
	return Stat{Uid: stat.Uid, Gid: stat.Gid}, ok
}

// SigInt is the value of SIGINT.
const SigInt = Signal(syscall.SIGINT)

// SigKill is the value of SIGKILL.
const SigKill = Signal(syscall.SIGKILL)

type Signal syscall.Signal

func SysKill(pid int, sig Signal) error {
	return syscall.Kill(pid, syscall.Signal(sig))
}
