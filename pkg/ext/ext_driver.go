// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ext

import (
	"github.com/lima-vm/lima/pkg/driver"
)

type LimaExtDriver struct {
	*driver.BaseDriver
}

func New(driver *driver.BaseDriver) *LimaExtDriver {
	return &LimaExtDriver{
		BaseDriver: driver,
	}
}
