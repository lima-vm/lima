package osutil

import (
	"fmt"
	"io/fs"
)

// UnixPathMax is the value of UNIX_PATH_MAX.
const UnixPathMax = 108

// Stat is a selection of syscall.Stat_t
type Stat struct {
	Uid uint32
	Gid uint32
}

func SysStat(fi fs.FileInfo) (Stat, bool) {
	return Stat{Uid: 0, Gid: 0}, false
}

// SigInt is the value of SIGINT.
const SigInt = Signal(2)

// SigKill is the value of SIGKILL.
const SigKill = Signal(9)

type Signal int

func SysKill(pid int, sig Signal) error {
	return fmt.Errorf("unimplemented")
}
