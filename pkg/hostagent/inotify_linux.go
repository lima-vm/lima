// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import "github.com/rjeczalik/notify"

func GetNotifyEvent() notify.Event {
	return notify.Create | notify.Write | notify.InAttrib
}
