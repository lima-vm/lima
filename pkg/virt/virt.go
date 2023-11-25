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

const connectString = "qemu:///session"

func virsh(args ...string) error {
	connect := []string{"--connect", connectString}
	args = append(connect, args...)
	return exec.Command("virsh", args...).Run()
}

func CreateNetwork(xml string) error {
	return virsh("net-create", xml)
}

func CreateDomain(xml string) error {
	return virsh("create", xml)
}
