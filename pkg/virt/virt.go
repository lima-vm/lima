package virt

import (
	"fmt"
	"os/exec"
	"regexp"
)

func Version() (string, error) {
	out, err := exec.Command("virsh", "version").Output()
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile("Using library: libvirt (.*)")
	m := re.FindStringSubmatch(string(out))
	if m == nil {
		return "", fmt.Errorf("can't find libvirt version")
	}
	return m[1], nil
}
