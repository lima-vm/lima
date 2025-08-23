// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package cidata

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/debugutil"
	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/instance/hostname"
	"github.com/lima-vm/lima/v2/pkg/iso9660util"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/localpathutil"
	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/networks/usernet"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
)

var netLookupIP = func(host string) []net.IP {
	ips, err := net.LookupIP(host)
	if err != nil {
		logrus.Debugf("net.LookupIP %s: %s", host, err)
		return nil
	}

	return ips
}

func setupEnv(instConfigEnv map[string]string, propagateProxyEnv bool, slirpGateway string) (map[string]string, error) {
	// Start with the proxy variables from the system settings.
	env, err := osutil.ProxySettings()
	if err != nil {
		return env, err
	}
	// env.* settings from lima.yaml override system settings without giving a warning
	maps.Copy(env, instConfigEnv)
	// Current process environment setting override both system settings and env.*
	lowerVars := []string{"ftp_proxy", "http_proxy", "https_proxy", "no_proxy"}
	upperVars := make([]string, len(lowerVars))
	for i, name := range lowerVars {
		upperVars[i] = strings.ToUpper(name)
	}
	if propagateProxyEnv {
		for _, name := range append(lowerVars, upperVars...) {
			if value, ok := os.LookupEnv(name); ok {
				if _, ok := env[name]; ok && value != env[name] {
					logrus.Infof("Overriding %q value %q with %q from limactl process environment",
						name, env[name], value)
				}
				env[name] = value
			}
		}
	}
	// Replace IP that IsLoopback in proxy settings with the gateway address
	// Delete settings with empty values, so the user can choose to ignore system settings.
	for _, name := range append(lowerVars, upperVars...) {
		value, ok := env[name]
		if ok && value == "" {
			delete(env, name)
		} else if ok && !strings.EqualFold(name, "no_proxy") {
			u, err := url.Parse(value)
			if err != nil {
				logrus.Warnf("Ignoring invalid proxy %q=%v: %s", name, value, err)
				continue
			}

			for _, ip := range netLookupIP(u.Hostname()) {
				if ip.IsLoopback() {
					newHost := slirpGateway
					if u.Port() != "" {
						newHost = net.JoinHostPort(newHost, u.Port())
					}
					u.Host = newHost
					value = u.String()
				}
			}
			if value != env[name] {
				logrus.Infof("Replacing %q value %q with %q", name, env[name], value)
				env[name] = value
			}
		}
	}
	// Make sure uppercase variants have the same value as lowercase ones.
	// If both are set, the lowercase variant value takes precedence.
	for _, lowerName := range lowerVars {
		upperName := strings.ToUpper(lowerName)
		if _, ok := env[lowerName]; ok {
			if _, ok := env[upperName]; ok && env[lowerName] != env[upperName] {
				logrus.Warnf("Changing %q value from %q to %q to match %q",
					upperName, env[upperName], env[lowerName], lowerName)
			}
			env[upperName] = env[lowerName]
		} else if _, ok := env[upperName]; ok {
			env[lowerName] = env[upperName]
		}
	}
	return env, nil
}

func templateArgs(ctx context.Context, bootScripts bool, instDir, name string, instConfig *limatype.LimaYAML, udpDNSLocalPort, tcpDNSLocalPort, vsockPort int, virtioPort string) (*TemplateArgs, error) {
	if err := limayaml.Validate(instConfig, false); err != nil {
		return nil, err
	}
	archive := "nerdctl-full.tgz"
	args := TemplateArgs{
		Debug:              debugutil.Debug,
		BootScripts:        bootScripts,
		Name:               name,
		Hostname:           hostname.FromInstName(name), // TODO: support customization
		User:               *instConfig.User.Name,
		Comment:            *instConfig.User.Comment,
		Home:               *instConfig.User.Home,
		Shell:              *instConfig.User.Shell,
		UID:                *instConfig.User.UID,
		GuestInstallPrefix: *instConfig.GuestInstallPrefix,
		UpgradePackages:    *instConfig.UpgradePackages,
		Containerd:         Containerd{System: *instConfig.Containerd.System, User: *instConfig.Containerd.User, Archive: archive},
		SlirpNICName:       networks.SlirpNICName,

		VMType:     *instConfig.VMType,
		VSockPort:  vsockPort,
		VirtioPort: virtioPort,
		Plain:      *instConfig.Plain,
		TimeZone:   *instConfig.TimeZone,
		Param:      instConfig.Param,
	}

	if instConfig.VMOpts.VZ.Rosetta.Enabled != nil {
		args.RosettaEnabled = *instConfig.VMOpts.VZ.Rosetta.Enabled
	}
	if instConfig.VMOpts.VZ.Rosetta.BinFmt != nil {
		args.RosettaEnabled = *instConfig.VMOpts.VZ.Rosetta.BinFmt
	}

	firstUsernetIndex := limayaml.FirstUsernetIndex(instConfig)
	var subnet net.IP
	var err error

	if firstUsernetIndex != -1 {
		usernetName := instConfig.Networks[firstUsernetIndex].Lima
		subnet, err = usernet.Subnet(usernetName)
		if err != nil {
			return nil, err
		}
		args.SlirpGateway = usernet.GatewayIP(subnet)
		args.SlirpDNS = usernet.GatewayIP(subnet)
	} else {
		subnet, _, err = net.ParseCIDR(networks.SlirpNetwork)
		if err != nil {
			return nil, err
		}
		args.SlirpGateway = usernet.GatewayIP(subnet)
		if *instConfig.VMType == limatype.VZ {
			args.SlirpDNS = usernet.GatewayIP(subnet)
		} else {
			args.SlirpDNS = usernet.DNSIP(subnet)
		}
		args.SlirpIPAddress = networks.SlirpIPAddress
	}

	// change instance id on every boot so network config will be processed again
	args.IID = fmt.Sprintf("iid-%d", time.Now().Unix())

	pubKeys, err := sshutil.DefaultPubKeys(ctx, *instConfig.SSH.LoadDotSSHPubKeys)
	if err != nil {
		return nil, err
	}
	if len(pubKeys) == 0 {
		return nil, errors.New("no SSH key was found, run `ssh-keygen`")
	}
	for _, f := range pubKeys {
		args.SSHPubKeys = append(args.SSHPubKeys, f.Content)
	}

	var fstype string
	switch *instConfig.MountType {
	case limatype.REVSSHFS:
		fstype = "sshfs"
	case limatype.NINEP:
		fstype = "9p"
	case limatype.VIRTIOFS:
		fstype = "virtiofs"
	}
	hostHome, err := localpathutil.Expand("~")
	if err != nil {
		return nil, err
	}
	for i, f := range instConfig.Mounts {
		tag := fmt.Sprintf("mount%d", i)
		options := "defaults"
		switch fstype {
		case "9p", "virtiofs":
			options = "ro"
			if *f.Writable {
				options = "rw"
			}
			if fstype == "9p" {
				options += ",trans=virtio"
				options += fmt.Sprintf(",version=%s", *f.NineP.ProtocolVersion)
				msize, err := units.RAMInBytes(*f.NineP.Msize)
				if err != nil {
					return nil, fmt.Errorf("failed to parse msize for %q: %w", f.Location, err)
				}
				options += fmt.Sprintf(",msize=%d", msize)
				options += fmt.Sprintf(",cache=%s", *f.NineP.Cache)
			}
			// don't fail the boot, if virtfs is not available
			options += ",nofail"
		}
		args.Mounts = append(args.Mounts, Mount{Tag: tag, MountPoint: *f.MountPoint, Type: fstype, Options: options})
		if f.Location == hostHome {
			args.HostHomeMountPoint = *f.MountPoint
		}
	}

	switch *instConfig.MountType {
	case limatype.REVSSHFS:
		args.MountType = "reverse-sshfs"
	case limatype.NINEP:
		args.MountType = "9p"
	case limatype.VIRTIOFS:
		args.MountType = "virtiofs"
	}

	for i, d := range instConfig.AdditionalDisks {
		format := true
		if d.Format != nil {
			format = *d.Format
		}
		fstype := ""
		if d.FSType != nil {
			fstype = *d.FSType
		}
		args.Disks = append(args.Disks, Disk{
			Name:   d.Name,
			Device: diskDeviceNameFromOrder(i),
			Format: format,
			FSType: fstype,
			FSArgs: d.FSArgs,
		})
	}

	args.Networks = append(args.Networks, Network{MACAddress: limayaml.MACAddress(instDir), Interface: networks.SlirpNICName, Metric: 200})
	for i, nw := range instConfig.Networks {
		if i == firstUsernetIndex {
			continue
		}
		args.Networks = append(args.Networks, Network{MACAddress: nw.MACAddress, Interface: nw.Interface, Metric: *nw.Metric})
	}

	args.Env, err = setupEnv(instConfig.Env, *instConfig.PropagateProxyEnv, args.SlirpGateway)
	if err != nil {
		return nil, err
	}

	switch {
	case len(instConfig.DNS) > 0:
		for _, addr := range instConfig.DNS {
			args.DNSAddresses = append(args.DNSAddresses, addr.String())
		}
	case firstUsernetIndex != -1 || *instConfig.VMType == limatype.VZ:
		args.DNSAddresses = append(args.DNSAddresses, args.SlirpDNS)
	case *instConfig.HostResolver.Enabled:
		args.UDPDNSLocalPort = udpDNSLocalPort
		args.TCPDNSLocalPort = tcpDNSLocalPort
		args.DNSAddresses = append(args.DNSAddresses, args.SlirpDNS)
	default:
		args.DNSAddresses, err = osutil.DNSAddresses()
		if err != nil {
			return nil, err
		}
	}

	args.CACerts.RemoveDefaults = instConfig.CACertificates.RemoveDefaults

	for _, path := range instConfig.CACertificates.Files {
		expanded, err := localpathutil.Expand(path)
		if err != nil {
			return nil, err
		}

		content, err := os.ReadFile(expanded)
		if err != nil {
			return nil, err
		}

		cert := getCert(string(content))
		args.CACerts.Trusted = append(args.CACerts.Trusted, cert)
	}

	for _, content := range instConfig.CACertificates.Certs {
		cert := getCert(content)
		args.CACerts.Trusted = append(args.CACerts.Trusted, cert)
	}

	// Remove empty caCerts (default values) from configuration yaml
	if !*args.CACerts.RemoveDefaults && len(args.CACerts.Trusted) == 0 {
		args.CACerts.RemoveDefaults = nil
		args.CACerts.Trusted = nil
	}

	args.BootCmds = getBootCmds(instConfig.Provision)

	for i, f := range instConfig.Provision {
		if f.Mode == limatype.ProvisionModeDependency && *f.SkipDefaultDependencyResolution {
			args.SkipDefaultDependencyResolution = true
		}
		if f.Mode == limatype.ProvisionModeData {
			args.DataFiles = append(args.DataFiles, DataFile{
				FileName:    fmt.Sprintf("%08d", i),
				Overwrite:   strconv.FormatBool(*f.Overwrite),
				Owner:       *f.Owner,
				Path:        *f.Path,
				Permissions: *f.Permissions,
			})
		}
	}

	return &args, nil
}

func GenerateCloudConfig(ctx context.Context, instDir, name string, instConfig *limatype.LimaYAML) error {
	args, err := templateArgs(ctx, false, instDir, name, instConfig, 0, 0, 0, "")
	if err != nil {
		return err
	}
	// mounts are not included here
	args.Mounts = nil
	// resolv_conf is not included here
	args.DNSAddresses = nil

	if err := ValidateTemplateArgs(args); err != nil {
		return err
	}

	config, err := ExecuteTemplateCloudConfig(args)
	if err != nil {
		return err
	}

	os.RemoveAll(filepath.Join(instDir, filenames.CloudConfig)) // delete existing
	return os.WriteFile(filepath.Join(instDir, filenames.CloudConfig), config, 0o444)
}

func GenerateISO9660(ctx context.Context, drv driver.Driver, instDir, name string, instConfig *limatype.LimaYAML, udpDNSLocalPort, tcpDNSLocalPort int, guestAgentBinary, nerdctlArchive string, vsockPort int, virtioPort string) error {
	args, err := templateArgs(ctx, true, instDir, name, instConfig, udpDNSLocalPort, tcpDNSLocalPort, vsockPort, virtioPort)
	if err != nil {
		return err
	}

	if err := ValidateTemplateArgs(args); err != nil {
		return err
	}

	layout, err := ExecuteTemplateCIDataISO(args)
	if err != nil {
		return err
	}

	driverScripts, err := drv.BootScripts()
	if err != nil {
		return fmt.Errorf("failed to get boot scripts: %w", err)
	}

	for filename, content := range driverScripts {
		layout = append(layout, iso9660util.Entry{
			Path:   fmt.Sprintf("boot/%s", filename),
			Reader: strings.NewReader(string(content)),
		})
	}

	for i, f := range instConfig.Provision {
		switch f.Mode {
		case limatype.ProvisionModeSystem, limatype.ProvisionModeUser, limatype.ProvisionModeDependency:
			layout = append(layout, iso9660util.Entry{
				Path:   fmt.Sprintf("provision.%s/%08d", f.Mode, i),
				Reader: strings.NewReader(f.Script),
			})
		case limatype.ProvisionModeData:
			layout = append(layout, iso9660util.Entry{
				Path:   fmt.Sprintf("provision.%s/%08d", f.Mode, i),
				Reader: strings.NewReader(*f.Content),
			})
		case limatype.ProvisionModeBoot:
			continue
		case limatype.ProvisionModeAnsible:
			continue
		default:
			return fmt.Errorf("unknown provision mode %q", f.Mode)
		}
	}

	if guestAgentBinary != "" {
		var guestAgent io.ReadCloser
		if strings.HasSuffix(guestAgentBinary, ".gz") {
			logrus.Debugf("Decompressing %s", guestAgentBinary)
			guestAgentGz, err := os.Open(guestAgentBinary)
			if err != nil {
				return err
			}
			defer guestAgentGz.Close()
			guestAgent, err = gzip.NewReader(guestAgentGz)
			if err != nil {
				return err
			}
		} else {
			guestAgent, err = os.Open(guestAgentBinary)
			if err != nil {
				return err
			}
		}

		defer guestAgent.Close()
		layout = append(layout, iso9660util.Entry{
			Path:   "lima-guestagent",
			Reader: guestAgent,
		})
	}

	if nerdctlArchive != "" {
		nftgz := args.Containerd.Archive
		nftgzR, err := os.Open(nerdctlArchive)
		if err != nil {
			return err
		}
		defer nftgzR.Close()
		layout = append(layout, iso9660util.Entry{
			// ISO9660 requires len(Path) <= 30
			Path:   nftgz,
			Reader: nftgzR,
		})
	}

	if args.VMType == limatype.WSL2 {
		layout = append(layout, iso9660util.Entry{
			Path:   "ssh_authorized_keys",
			Reader: strings.NewReader(strings.Join(args.SSHPubKeys, "\n")),
		})
		return writeCIDataDir(filepath.Join(instDir, filenames.CIDataISODir), layout)
	}

	return iso9660util.Write(filepath.Join(instDir, filenames.CIDataISO), "cidata", layout)
}

func getCert(content string) Cert {
	lines := []string{}
	for _, line := range strings.Split(content, "\n") {
		if line == "" {
			continue
		}
		lines = append(lines, strings.TrimSpace(line))
	}
	// return lines
	return Cert{Lines: lines}
}

func getBootCmds(p []limatype.Provision) []BootCmds {
	var bootCmds []BootCmds
	for _, f := range p {
		if f.Mode == limatype.ProvisionModeBoot {
			lines := []string{}
			for _, line := range strings.Split(f.Script, "\n") {
				if line == "" {
					continue
				}
				lines = append(lines, strings.TrimSpace(line))
			}
			bootCmds = append(bootCmds, BootCmds{Lines: lines})
		}
	}
	return bootCmds
}

func diskDeviceNameFromOrder(order int) string {
	return fmt.Sprintf("vd%c", int('b')+order)
}

func writeCIDataDir(rootPath string, layout []iso9660util.Entry) error {
	slices.SortFunc(layout, func(a, b iso9660util.Entry) int {
		return strings.Compare(strings.ToLower(a.Path), strings.ToLower(b.Path))
	})

	if err := os.RemoveAll(rootPath); err != nil {
		return err
	}

	for _, e := range layout {
		if dir := path.Dir(e.Path); dir != "" && dir != "/" {
			if err := os.MkdirAll(filepath.Join(rootPath, dir), 0o700); err != nil {
				return err
			}
		}
		f, err := os.OpenFile(filepath.Join(rootPath, e.Path), os.O_CREATE|os.O_RDWR, 0o700)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, e.Reader); err != nil {
			_ = f.Close()
			return err
		}
		_ = f.Close()
	}

	return nil
}
