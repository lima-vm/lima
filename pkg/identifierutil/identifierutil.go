package identifierutil

import "strings"

func HostnameFromInstName(instName string) string {
	s := strings.ReplaceAll(instName, ".", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return "lima-" + s
}
