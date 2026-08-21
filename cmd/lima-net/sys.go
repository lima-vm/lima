//go:build linux

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

const sysClassNet = "/sys/class/net"

// safeDirs are the only directories the privileged commands are looked up in.
// This process runs as root, so resolving them through the PATH inherited from
// the unprivileged caller would hand that caller arbitrary root code execution
// on any host where sudo has no secure_path.
var safeDirs = []string{"/usr/sbin", "/usr/bin", "/sbin", "/bin"}

func lookTool(name string) (string, error) {
	if strings.ContainsRune(name, os.PathSeparator) {
		return "", fmt.Errorf("invalid command name %#q", name)
	}
	for _, dir := range safeDirs {
		path := filepath.Join(dir, name)
		if fi, err := os.Stat(path); err == nil && fi.Mode().IsRegular() && fi.Mode()&0o111 != 0 {
			return path, nil
		}
	}
	return "", fmt.Errorf("command %#q not found in %v", name, safeDirs)
}

// command builds an *exec.Cmd for a tool resolved to an absolute path.
func command(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	path, err := lookTool(name)
	if err != nil {
		return nil, err
	}
	return exec.CommandContext(ctx, path, args...), nil
}

// run executes a command and includes its output in the error, if any.
func run(ctx context.Context, name string, args ...string) error {
	cmd, err := command(ctx, name, args...)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	logrus.Debugf("Running: %v", cmd.Args)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %v: %w (output: %s)", cmd.Args, err, strings.TrimSpace(out.String()))
	}
	return nil
}

// output runs a command and returns its standard output.
func output(ctx context.Context, name string, args ...string) (string, error) {
	cmd, err := command(ctx, name, args...)
	if err != nil {
		return "", err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	logrus.Debugf("Running: %v", cmd.Args)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run %v: %w (output: %s)", cmd.Args, err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// runOK reports whether the command exited with status 0. It is used for the
// probing commands (`iptables -C`, `firewall-cmd --state`) whose non-zero exit
// status is an expected answer rather than a failure.
func runOK(ctx context.Context, name string, args ...string) bool {
	cmd, err := command(ctx, name, args...)
	if err != nil {
		logrus.WithError(err).Debugf("Cannot probe with %#q", name)
		return false
	}
	logrus.Debugf("Probing: %v", cmd.Args)
	return cmd.Run() == nil
}

func ufwActive(ctx context.Context) bool {
	cmd, err := command(ctx, "ufw", "status")
	if err != nil {
		return false
	}
	out, err := cmd.Output()
	return err == nil && strings.Contains(string(out), "Status: active")
}

// iptablesRules adds or deletes rules, unless they are already in the desired
// state. Every rule is "<table> <chain> <args>...". addOp is "-A" to append or
// "-I" to insert at the top; inserted rules are applied in reverse, so that the
// resulting order in the chain matches the order they are listed in here.
func iptablesRules(ctx context.Context, add bool, addOp string, rules ...[]string) error {
	if add && addOp == "-I" {
		rules = slices.Clone(rules)
		slices.Reverse(rules)
	}
	var errs []error
	for _, rule := range rules {
		table, chain, args := rule[0], rule[1], rule[2:]
		if runOK(ctx, "iptables", append([]string{"-w", "-t", table, "-C", chain}, args...)...) == add {
			continue
		}
		op := "-D"
		if add {
			op = addOp
		}
		errs = append(errs, run(ctx, "iptables", append([]string{"-w", "-t", table, op, chain}, args...)...))
	}
	return errors.Join(errs...)
}

func interfaceExists(name string) bool {
	_, err := os.Lstat(filepath.Join(sysClassNet, name))
	return err == nil
}

func isBridge(name string) bool {
	st, err := os.Stat(filepath.Join(sysClassNet, name, "bridge"))
	return err == nil && st.IsDir()
}

// tapOwner returns the uid a tun/tap device was created for, or -1 when it has
// no owner (which the caller must treat as "not ours").
func tapOwner(name string) (int64, error) {
	b, err := os.ReadFile(filepath.Join(sysClassNet, name, "owner"))
	if err != nil {
		return -1, fmt.Errorf("interface %#q is not a tap device: %w", name, err)
	}
	owner, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return -1, fmt.Errorf("cannot parse the owner of %#q: %w", name, err)
	}
	return owner, nil
}

func ipForwardEnabled() bool {
	b, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	return err == nil && strings.TrimSpace(string(b)) == "1"
}

// ensureRunDir creates the directory holding the PID file and verifies that only
// root can write to it: a user-writable PID file would let `pkill -F` kill any
// process as root.
func ensureRunDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// MkdirAll leaves a pre-existing directory alone, so the mode is not implied.
	if err := os.Chmod(dir, 0o755); err != nil {
		return err
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("runtime directory %#q is a symlink", dir)
	}
	st, ok := osutil.SysStat(fi)
	if !ok {
		return fmt.Errorf("could not retrieve stat buffer for %#q", dir)
	}
	if st.Uid != 0 {
		return fmt.Errorf("runtime directory %#q is owned by uid %d, not by root", dir, st.Uid)
	}
	if fi.Mode()&0o022 != 0 {
		return fmt.Errorf("runtime directory %#q is writable by non-root users (mode %o)", dir, fi.Mode().Perm())
	}
	return nil
}

// bridgePorts returns the names of the interfaces enslaved to the given bridge.
func bridgePorts(bridge string) []string {
	ents, err := os.ReadDir(filepath.Join(sysClassNet, bridge, "brif"))
	if err != nil {
		return nil
	}
	ports := make([]string, 0, len(ents))
	for _, ent := range ents {
		ports = append(ports, ent.Name())
	}
	return ports
}

// otherLimaBridges returns the Lima-managed bridges other than the given one
// that are still present on the host.
func otherLimaBridges(except string) []string {
	ents, err := os.ReadDir(sysClassNet)
	if err != nil {
		return nil
	}
	var bridges []string
	for _, ent := range ents {
		if name := ent.Name(); name != except && networks.IsManagedBridge(name) && isBridge(name) {
			bridges = append(bridges, name)
		}
	}
	return bridges
}

// interfaceMaster returns the name of the bridge the interface is enslaved to,
// or "" when it is not enslaved to any.
func interfaceMaster(name string) string {
	link, err := os.Readlink(filepath.Join(sysClassNet, name, "master"))
	if err != nil {
		return ""
	}
	return filepath.Base(link)
}

func interfaceIsUp(name string) bool {
	iface, err := net.InterfaceByName(name)
	return err == nil && iface.Flags&net.FlagUp != 0
}

func interfaceHasAddress(name string, ip net.IP) bool {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return false
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.Equal(ip) {
			return true
		}
	}
	return false
}
