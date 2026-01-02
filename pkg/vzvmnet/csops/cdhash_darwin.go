// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package csops

/*
#include <errno.h>
#include <string.h>
#include <unistd.h>

// see: https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/bsd/sys/codesign.h#L72
int csops(pid_t pid, unsigned int ops, void *useraddr, size_t usersize);

// see: https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/bsd/sys/codesign.h#L48
#define CS_OPS_CDHASH 5

enum {
// see: https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/osfmk/kern/cs_blobs.h#L142
	CS_CDHASH_LEN = 20,
};

*/
import (
	"C" //nolint:gocritic // false positive: dupImport: package is imported 2 times under different aliases on... (gocritic)
)

import (
	"fmt"
	"os"
	"unsafe" //nolint:gocritic // false positive: dupImport: package is imported 2 times under different aliases on... (gocritic)
)

// Cdhash retrieves the CDHash of the process with the given PID using csops.
// Returns a byte slice containing the CDHash or an error if the operation fails.
// The CDHash is a unique identifier for the code signature of the executable.
//
// CDHash can also be obtained from an executable using the following command:
//
//	codesign --display -vvv <executable> 2>&1 | grep 'CDHash='
func Cdhash(pid int) ([]byte, error) {
	buf := make([]byte, C.CS_CDHASH_LEN)
	r, err := C.csops(
		C.pid_t(pid),
		C.CS_OPS_CDHASH,
		unsafe.Pointer(&buf[0]),
		C.size_t(len(buf)),
	)
	if r != 0 {
		return nil, fmt.Errorf("csops failed: %w", err)
	}
	return buf, nil
}

// SelfCdhash retrieves the CDHash of the current process using csops.
// Returns a byte slice containing the CDHash or an error if the operation fails.
// The CDHash is a unique identifier for the code signature of the executable.
func SelfCdhash() ([]byte, error) {
	return Cdhash(os.Getpid())
}
