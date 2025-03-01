// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package identifierutil

import "strings"

func HostnameFromInstName(instName string) string {
	s := strings.ReplaceAll(instName, ".", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return "lima-" + s
}
