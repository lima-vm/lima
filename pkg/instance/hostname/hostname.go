// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostname

import "strings"

// FromInstName generates a hostname from an instance name by prefixing
// it with "lima-" and replacing all dots and underscores with dashes.
// E.g. "my_example.com" becomes "lima-my-example-com".
func FromInstName(instName string) string {
	s := strings.ReplaceAll(instName, ".", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return "lima-" + s
}
