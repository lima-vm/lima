//go:build linux

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

package fsutil

import (
	"golang.org/x/sys/unix"
)

func IsNFS(path string) (bool, error) {
	var sf unix.Statfs_t
	if err := unix.Statfs(path, &sf); err != nil {
		return false, err
	}
	return sf.Type == unix.NFS_SUPER_MAGIC, nil
}
