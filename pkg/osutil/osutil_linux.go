/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
