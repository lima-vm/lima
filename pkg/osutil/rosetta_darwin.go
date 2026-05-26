// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func IsBeingRosettaTranslated() bool {
	ret, err := unix.SysctlUint32("sysctl.proc_translated")
	if err != nil {
		const fallback = false
		if errors.Is(err, unix.ENOENT) {
			return false
		}

		err = fmt.Errorf(`failed to read sysctl "sysctl.proc_translated": %w`, err)
		logrus.WithError(err).Warnf("failed to detect whether running under rosetta, assuming %v", fallback)
		return fallback
	}

	return ret != 0
}
