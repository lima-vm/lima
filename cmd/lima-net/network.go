//go:build linux

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/networks"
)

// netConfig is the fully resolved description of a Lima network. Every value is
// passed on the command line so that the sudoers file can pin it; lima-net never
// reads the (user-writable) networks.yaml itself.
type netConfig struct {
	pidFile string
	mode    string
	bridge  string
	gateway string
	netmask string
	dhcpEnd string

	// parsed by validate
	ip     net.IP
	mask   net.IPMask
	end    net.IP
	subnet *net.IPNet

	enabledForwarding bool
}

// validIfName reports whether name is a valid network interface name: at most 15
// characters (IFNAMSIZ-1) of the characters that both the kernel and a command line accept.
func validIfName(name string) bool {
	if name == "" || len(name) > 15 || name == "." || name == ".." {
		return false
	}
	// A leading dash would be parsed as an option by `ip`.
	if name[0] == '-' {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

func (c *netConfig) validate() error {
	if !filepath.IsAbs(c.pidFile) || strings.ContainsAny(c.pidFile, " \t\n") {
		return fmt.Errorf("invalid pidfile %#q", c.pidFile)
	}
	if !validIfName(c.bridge) {
		return fmt.Errorf("invalid bridge name %#q", c.bridge)
	}
	if c.mode == networks.ModeBridged {
		return nil
	}
	if c.mode != networks.ModeShared && c.mode != networks.ModeHost {
		return fmt.Errorf("unsupported mode %#q", c.mode)
	}
	if c.ip = net.ParseIP(c.gateway).To4(); c.ip == nil {
		return fmt.Errorf("gateway %#q is not an IPv4 address", c.gateway)
	}
	mask := net.ParseIP(c.netmask).To4()
	if mask == nil {
		return fmt.Errorf("netmask %#q is not an IPv4 address", c.netmask)
	}
	c.mask = net.IPMask(mask)
	if ones, bits := c.mask.Size(); bits != 32 || ones < 8 || ones > 30 {
		return fmt.Errorf("netmask %#q is not a valid IPv4 netmask", c.netmask)
	}
	if c.end = net.ParseIP(c.dhcpEnd).To4(); c.end == nil {
		return fmt.Errorf("dhcpEnd %#q is not an IPv4 address", c.dhcpEnd)
	}
	c.subnet = &net.IPNet{IP: c.ip.Mask(c.mask), Mask: c.mask}
	if !c.subnet.Contains(c.end) {
		return fmt.Errorf("dhcpEnd %v is outside of subnet %v", c.end, c.subnet)
	}
	return nil
}

// run brings the network up and then blocks until limactl terminates the process
// (via `pkill -F <pidfile>`).
func (c *netConfig) run(ctx context.Context) error {
	if err := c.validate(); err != nil {
		return err
	}
	if err := ensureRunDir(filepath.Dir(c.pidFile)); err != nil {
		return err
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// Teardown must run to completion even if ctx is already done, or the bridge
	// and the firewall rules would be left behind on the host.
	downCtx := context.WithoutCancel(ctx)

	if err := c.up(ctx); err != nil {
		return errors.Join(err, c.down(downCtx))
	}
	dnsmasq, err := c.startDnsmasq(ctx)
	if err != nil {
		return errors.Join(err, c.down(downCtx))
	}
	if err := os.WriteFile(c.pidFile, fmt.Appendf(nil, "%d\n", os.Getpid()), 0o644); err != nil {
		return errors.Join(err, stopDnsmasq(dnsmasq), c.down(downCtx))
	}
	logrus.Infof("Network is up on bridge %#q", c.bridge)

	<-sig
	logrus.Infof("Shutting down the network on bridge %#q", c.bridge)
	return errors.Join(os.Remove(c.pidFile), stopDnsmasq(dnsmasq), c.down(downCtx))
}

// up creates the bridge and, for "shared" networks, the NAT rules. Every step is
// skipped when it has already been performed.
func (c *netConfig) up(ctx context.Context) error {
	if c.mode == networks.ModeBridged {
		// The bridge belongs to the host administrator: only verify it, never touch it.
		if !isBridge(c.bridge) {
			return fmt.Errorf("%#q is not an existing bridge: the `interface` of a %#q network must be a bridge created by the host administrator",
				c.bridge, networks.ModeBridged)
		}
		return nil
	}
	if err := c.ensureBridge(ctx); err != nil {
		return err
	}
	if err := c.firewall(ctx, true); err != nil {
		return err
	}
	return c.forwarding(ctx, true)
}

// down removes everything up() created, plus the tap devices that Lima attached
// to the bridge. Every step is attempted so that one failure does not strand the
// rest.
func (c *netConfig) down(ctx context.Context) error {
	var errs []error
	for _, port := range bridgePorts(c.bridge) {
		if networks.IsTapName(port) {
			errs = append(errs, run(ctx, "ip", "link", "delete", port))
		}
	}
	if c.mode == networks.ModeBridged {
		return errors.Join(errs...)
	}
	errs = append(errs, c.firewall(ctx, false))
	if interfaceExists(c.bridge) {
		errs = append(errs, run(ctx, "ip", "link", "delete", c.bridge))
	}
	// After the bridge is gone, so that restoreForwarding() no longer counts it.
	errs = append(errs, c.forwarding(ctx, false))
	return errors.Join(errs...)
}

func (c *netConfig) ensureBridge(ctx context.Context) error {
	if interfaceExists(c.bridge) {
		if !isBridge(c.bridge) {
			return fmt.Errorf("interface %#q already exists but is not a bridge", c.bridge)
		}
	} else if err := run(ctx, "ip", "link", "add", "name", c.bridge, "type", "bridge"); err != nil {
		return err
	}
	if !interfaceHasAddress(c.bridge, c.ip) {
		ones, _ := c.mask.Size()
		if err := run(ctx, "ip", "address", "add", fmt.Sprintf("%s/%d", c.ip, ones), "dev", c.bridge); err != nil {
			return err
		}
	}
	if interfaceIsUp(c.bridge) {
		return nil
	}
	return run(ctx, "ip", "link", "set", "dev", c.bridge, "up")
}

// forwarding adds or removes the FORWARD (and, for "shared", the NAT) rules of
// the network. They are needed even when a firewall manager already accepts the
// bridge's traffic, because net.ipv4.ip_forward is global: once any "shared"
// network has enabled it, the host routes between all of its bridges.
func (c *netConfig) forwarding(ctx context.Context, add bool) error {
	if c.mode == networks.ModeHost {
		// An isolated network reaches the host itself (INPUT, not FORWARD) and the
		// guests on its own bridge, and nothing else. Inserted at the top so that
		// these rules win over the ACCEPT rules of a "shared" network, no matter
		// which network was started first.
		return iptablesRules(ctx, add, "-I",
			[]string{"filter", "FORWARD", "-i", c.bridge, "-o", c.bridge, "-j", "ACCEPT"},
			[]string{"filter", "FORWARD", "-i", c.bridge, "-j", "REJECT"},
			[]string{"filter", "FORWARD", "-o", c.bridge, "-j", "REJECT"},
		)
	}
	if add {
		if !ipForwardEnabled() {
			if err := run(ctx, "sysctl", "-q", "-w", "net.ipv4.ip_forward=1"); err != nil {
				return err
			}
			c.enabledForwarding = true
		}
	} else {
		defer c.restoreForwarding(ctx)
	}
	// Appended rather than inserted, so that the reject rules of an isolated
	// network stay above them.
	subnet := c.subnet.String()
	return iptablesRules(ctx, add, "-A",
		[]string{"nat", "POSTROUTING", "-s", subnet, "!", "-d", subnet, "-j", "MASQUERADE"},
		// The FORWARD policy is DROP on hosts running Docker, so the bridge needs
		// explicit permission in both directions.
		[]string{"filter", "FORWARD", "-i", c.bridge, "-s", subnet, "-j", "ACCEPT"},
		[]string{"filter", "FORWARD", "-o", c.bridge, "-d", subnet, "-j", "ACCEPT"},
	)
}

// restoreForwarding turns net.ipv4.ip_forward back off, but only when this
// process turned it on and no other Lima bridge is left that might still need
// it. Leaving it on would silently keep the host a router after teardown.
func (c *netConfig) restoreForwarding(ctx context.Context) {
	if !c.enabledForwarding {
		return
	}
	if others := otherLimaBridges(c.bridge); len(others) > 0 {
		logrus.Debugf("Leaving net.ipv4.ip_forward enabled: %v are still up", others)
		return
	}
	if err := run(ctx, "sysctl", "-q", "-w", "net.ipv4.ip_forward=0"); err != nil {
		logrus.WithError(err).Warn("Failed to restore net.ipv4.ip_forward")
	}
}

// firewallZones are the firewalld zones the Lima bridge is bound to, in order of
// preference. They all have target=ACCEPT, so the guests' forwarded traffic
// passes, while a low priority reject rule leaves only DHCP, DNS and ICMP
// reachable on the host itself. "lima" is not shipped by firewalld and only
// exists when an administrator (or ensureFirewallZone) created it; the other two
// are built in. The "trusted" zone is deliberately not used: it would expose
// every service listening on the host to the guests.
var firewallZones = []string{"lima", "nm-shared", "libvirt"}

// firewalldZone returns the zone to bind the bridge to, creating "lima" when the
// host has none of the built-in ones.
func firewalldZone(ctx context.Context) (string, error) {
	out, err := output(ctx, "firewall-cmd", "--get-zones")
	if err != nil {
		return "", err
	}
	existing := strings.Fields(out)
	for _, zone := range firewallZones {
		if slices.Contains(existing, zone) {
			return zone, nil
		}
	}
	return firewallZones[0], ensureFirewallZone(ctx, firewallZones[0])
}

// ensureFirewallZone creates a zone modelled on the built-in libvirt zone. New
// zones can only be defined permanently, so firewalld has to be reloaded once
// for the zone to become effective.
func ensureFirewallZone(ctx context.Context, zone string) error {
	permanent := []string{"--permanent", "--zone=" + zone}
	steps := [][]string{
		{"--permanent", "--new-zone=" + zone},
		append(slices.Clone(permanent), "--set-target=ACCEPT"),
		append(slices.Clone(permanent), "--add-service=dhcp", "--add-service=dns"),
		append(slices.Clone(permanent), "--add-protocol=icmp", "--add-protocol=ipv6-icmp"),
		append(slices.Clone(permanent), "--add-rich-rule=rule priority=\"32767\" reject"),
		{"--reload"},
	}
	logrus.Infof("Creating the %#q firewalld zone", zone)
	for _, step := range steps {
		if err := run(ctx, "firewall-cmd", step...); err != nil {
			return err
		}
	}
	return nil
}

// firewall opens the guest's DHCP (UDP 67) and DNS (port 53) packets towards the
// host. Without this, firewalld and ufw silently reject them even though they
// are visible on the bridge. Nothing else on the host is exposed.
func (c *netConfig) firewall(ctx context.Context, add bool) error {
	switch {
	case runOK(ctx, "firewall-cmd", "--state"):
		zone, err := firewalldZone(ctx)
		if err != nil {
			return err
		}
		action := "--change-interface=" + c.bridge
		if !add {
			action = "--remove-interface=" + c.bridge
		}
		// The permanent binding only survives a firewalld reload while the
		// interface exists, so a failure to persist it must not be fatal.
		for _, permanent := range []bool{false, true} {
			args := []string{"--zone=" + zone}
			if permanent {
				args = append(args, "--permanent")
			}
			// Removing an interface that is not bound is an error, and down() runs
			// even when up() never got as far as binding it.
			if !add && !runOK(ctx, "firewall-cmd", append(slices.Clone(args), "--query-interface="+c.bridge)...) {
				continue
			}
			if err := run(ctx, "firewall-cmd", append(args, action)...); err != nil {
				if !permanent {
					return err
				}
				logrus.WithError(err).Debugf("Failed to persist the firewalld zone of %#q", c.bridge)
			}
		}
		return nil
	case ufwActive(ctx):
		var errs []error
		for _, rule := range [][]string{
			{"in", "on", c.bridge, "to", "any", "port", "67", "proto", "udp"},
			{"in", "on", c.bridge, "to", "any", "port", "53", "proto", "udp"},
			{"in", "on", c.bridge, "to", "any", "port", "53", "proto", "tcp"},
		} {
			args := []string{"--force"}
			if !add {
				args = append(args, "delete")
			}
			args = append(args, "allow")
			errs = append(errs, run(ctx, "ufw", append(args, rule...)...))
		}
		return errors.Join(errs...)
	default:
		// No firewall manager: open just DHCP and DNS on the bridge.
		return iptablesRules(ctx, add, "-I",
			[]string{"filter", "INPUT", "-i", c.bridge, "-p", "udp", "--dport", "67", "-j", "ACCEPT"},
			[]string{"filter", "INPUT", "-i", c.bridge, "-p", "udp", "--dport", "53", "-j", "ACCEPT"},
			[]string{"filter", "INPUT", "-i", c.bridge, "-p", "tcp", "--dport", "53", "-j", "ACCEPT"},
		)
	}
}

// startDnsmasq starts the DHCP and DNS server as a child process, so that it is
// terminated together with lima-net instead of leaking.
func (c *netConfig) startDnsmasq(ctx context.Context) (*exec.Cmd, error) {
	if c.mode == networks.ModeBridged {
		return nil, nil // DHCP is provided by the outside network
	}
	args := []string{
		"--keep-in-foreground", // stay a child of this process
		"--conf-file=/dev/null",
		"--bind-dynamic",
		"--except-interface=lo",
		"--interface=" + c.bridge,
		"--log-facility=-", // log to stderr, which limactl captures
		"--dhcp-leasefile=" + strings.TrimSuffix(c.pidFile, ".pid") + ".leases",
		fmt.Sprintf("--dhcp-range=%s,%s,%s,1h", nextIP(c.ip), c.end, net.IP(c.mask)),
		"--dhcp-authoritative",
		"--dhcp-option=option:dns-server," + c.ip.String(),
	}
	if c.mode == networks.ModeHost {
		// An isolated network has no route to the outside, so advertise no default
		// route and never forward queries to the upstream resolvers.
		args = append(args, "--dhcp-option=option:router", "--no-resolv")
	} else {
		args = append(args, "--dhcp-option=option:router,"+c.ip.String())
	}
	cmd, err := command(ctx, "dnsmasq", args...)
	if err != nil {
		return nil, err
	}
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr

	// Without this, a crash of lima-net leaves a root dnsmasq behind.
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGTERM

	logrus.Debugf("Starting: %v", cmd.Args)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to run %v: %w", cmd.Args, err)
	}
	return cmd, nil
}

func stopDnsmasq(cmd *exec.Cmd) error {
	if cmd == nil {
		return nil
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		logrus.WithError(err).Debug("dnsmasq exited with an error")
	}
	return nil
}

// tapUp creates the tap device connecting an instance to the bridge, owned by the
// user who invoked sudo so that QEMU can open it without any privileges.
func tapUp(ctx context.Context, tap, bridge string) error {
	if !networks.IsTapName(tap) {
		return fmt.Errorf("%#q is not a Lima tap device name", tap)
	}
	if !validIfName(bridge) || !isBridge(bridge) {
		return fmt.Errorf("%#q is not an existing bridge", bridge)
	}
	// SUDO_UID is set by sudo itself, so the caller cannot forge it.
	uid, err := strconv.ParseUint(os.Getenv("SUDO_UID"), 10, 32)
	if err != nil || uid == 0 {
		return fmt.Errorf("invalid SUDO_UID %#q in the environment", os.Getenv("SUDO_UID"))
	}
	if interfaceExists(tap) {
		// Another user may own a device of the same name; never hand it to this
		// caller, and never take it away from its owner.
		owner, err := tapOwner(tap)
		if err != nil {
			return err
		}
		if owner != int64(uid) {
			return fmt.Errorf("tap device %#q is owned by uid %d, not by uid %d", tap, owner, uid)
		}
	} else if err := run(ctx, "ip", "tuntap", "add", "dev", tap, "mode", "tap", "user", strconv.FormatUint(uid, 10)); err != nil {
		return err
	}
	if interfaceMaster(tap) != bridge {
		if err := run(ctx, "ip", "link", "set", "dev", tap, "master", bridge); err != nil {
			return err
		}
	}
	if interfaceIsUp(tap) {
		return nil
	}
	return run(ctx, "ip", "link", "set", "dev", tap, "up")
}

// nextIP returns the address right after ip.
func nextIP(ip net.IP) net.IP {
	next := slices.Clone(ip.To4())
	next[3]++
	return next
}
