// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package systemd

import (
	"github.com/coreos/go-systemd/v22/util"
	"github.com/sirupsen/logrus"
)

func CurrentUnitName() string {
	unit, err := util.CurrentUnitName()
	if err != nil {
		logrus.WithError(err).Debug("cannot determine current systemd unit name")
	}
	return unit
}

func IsRunningSystemd() bool {
	return util.IsRunningSystemd()
}
