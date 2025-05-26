// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package instance

import "strings"

// HostnameFromInstName generates a hostname from an instance name by prefixing
// it with "lima-" and replacing all dots and underscores with dashes.
// E.g. "my_example.com" becomes "lima-my-example-com".
func HostnameFromInstName(instName string) string {
	s := strings.ReplaceAll(instName, ".", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return "lima-" + s
}
