package cidata

import (
	"errors"
	"fmt"
	"io"
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
	"github.com/lima-vm/lima/pkg/iso9660util"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/localpathutil"
	"github.com/lima-vm/lima/pkg/networks"
	"github.com/lima-vm/lima/pkg/networks/usernet"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/sshutil"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/lima-vm/lima/pkg/usrlocalsharelima"
	"github.com/sirupsen/logrus"
)

var netLookupIP = func(host string) []net.IP {
	ips, err := net.LookupIP(host)
	if err != nil {
		logrus.Debugf("net.LookupIP %s: %s", host, err)
		return nil
	}

	return ips
}

func setupEnv(y *limayaml.LimaYAML, args TemplateArgs) (map[string]string, error) {
	// Start with the proxy variables from the system settings.
	env, err := osutil.ProxySettings()
	if err != nil {
		return env, err
	}
	// env.* settings from lima.yaml override system settings without giving a warning
	for name, value := range y.Env {
		env[name] = value
	}
	// Current process environment setting override both system settings and env.*
	lowerVars := []string{"ftp_proxy", "http_proxy", "https_proxy", "no_proxy"}
	upperVars := make([]string, len(lowerVars))
	for i, name := range lowerVars {
		upperVars[i] = strings.ToUpper(name)
	}
	if *y.PropagateProxyEnv {
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
					newHost := args.SlirpGateway
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

func GenerateISO9660(instDir, name string, y *limayaml.LimaYAML, udpDNSLocalPort, tcpDNSLocalPort int, nerdctlArchive string, vsockPort int, virtioPort string) error {
	if err := limayaml.Validate(y, false); err != nil {
		return err
	}
	u, err := osutil.LimaUser(true)
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}
	args := TemplateArgs{
		Name:               name,
		User:               u.Username,
		UID:                uid,
		Home:               fmt.Sprintf("/home/%s.linux", u.Username),
		GuestInstallPrefix: *y.GuestInstallPrefix,
		UpgradePackages:    *y.UpgradePackages,
		Containerd:         Containerd{System: *y.Containerd.System, User: *y.Containerd.User},
		SlirpNICName:       networks.SlirpNICName,

		RosettaEnabled: *y.Rosetta.Enabled,
		RosettaBinFmt:  *y.Rosetta.BinFmt,
		VMType:         *y.VMType,
		VSockPort:      vsockPort,
		VirtioPort:     virtioPort,
		Plain:          *y.Plain,
		TimeZone:       *y.TimeZone,
	}

	firstUsernetIndex := limayaml.FirstUsernetIndex(y)
	var subnet net.IP

	if firstUsernetIndex != -1 {
		usernetName := y.Networks[firstUsernetIndex].Lima
		subnet, err = usernet.Subnet(usernetName)
		if err != nil {
			return err
		}
		args.SlirpGateway = usernet.GatewayIP(subnet)
		args.SlirpDNS = usernet.GatewayIP(subnet)
	} else {
		subnet, _, err = net.ParseCIDR(networks.SlirpNetwork)
		if err != nil {
			return err
		}
		args.SlirpGateway = usernet.GatewayIP(subnet)
		if *y.VMType == limayaml.VZ {
			args.SlirpDNS = usernet.GatewayIP(subnet)
		} else {
			args.SlirpDNS = usernet.DNSIP(subnet)
		}
		args.SlirpIPAddress = networks.SlirpIPAddress
	}

	// change instance id on every boot so network config will be processed again
	args.IID = fmt.Sprintf("iid-%d", time.Now().Unix())

	pubKeys, err := sshutil.DefaultPubKeys(*y.SSH.LoadDotSSHPubKeys)
	if err != nil {
		return err
	}
	if len(pubKeys) == 0 {
		return errors.New("no SSH key was found, run `ssh-keygen`")
	}
	for _, f := range pubKeys {
		args.SSHPubKeys = append(args.SSHPubKeys, f.Content)
	}

	var fstype string
	switch *y.MountType {
	case limayaml.REVSSHFS:
		fstype = "sshfs"
	case limayaml.NINEP:
		fstype = "9p"
	case limayaml.VIRTIOFS:
		fstype = "virtiofs"
	}
	hostHome, err := localpathutil.Expand("~")
	if err != nil {
		return err
	}
	for i, f := range y.Mounts {
		tag := fmt.Sprintf("mount%d", i)
		location, err := localpathutil.Expand(f.Location)
		if err != nil {
			return err
		}
		mountPoint, err := localpathutil.Expand(f.MountPoint)
		if err != nil {
			return err
		}
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
					return fmt.Errorf("failed to parse msize for %q: %w", location, err)
				}
				options += fmt.Sprintf(",msize=%d", msize)
				options += fmt.Sprintf(",cache=%s", *f.NineP.Cache)
			}
			// don't fail the boot, if virtfs is not available
			options += ",nofail"
		}
		args.Mounts = append(args.Mounts, Mount{Tag: tag, MountPoint: mountPoint, Type: fstype, Options: options})
		if location == hostHome {
			args.HostHomeMountPoint = mountPoint
		}
	}

	switch *y.MountType {
	case limayaml.REVSSHFS:
		args.MountType = "reverse-sshfs"
	case limayaml.NINEP:
		args.MountType = "9p"
	case limayaml.VIRTIOFS:
		args.MountType = "virtiofs"
	}

	for i, d := range y.AdditionalDisks {
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

	args.Networks = append(args.Networks, Network{MACAddress: limayaml.MACAddress(instDir), Interface: networks.SlirpNICName})
	for i, nw := range y.Networks {
		if i == firstUsernetIndex {
			continue
		}
		args.Networks = append(args.Networks, Network{MACAddress: nw.MACAddress, Interface: nw.Interface})
	}

	args.Env, err = setupEnv(y, args)
	if err != nil {
		return err
	}

	if len(y.DNS) > 0 {
		for _, addr := range y.DNS {
			args.DNSAddresses = append(args.DNSAddresses, addr.String())
		}
	} else if firstUsernetIndex != -1 || *y.VMType == limayaml.VZ {
		args.DNSAddresses = append(args.DNSAddresses, args.SlirpDNS)
	} else if *y.HostResolver.Enabled {
		args.UDPDNSLocalPort = udpDNSLocalPort
		args.TCPDNSLocalPort = tcpDNSLocalPort
		args.DNSAddresses = append(args.DNSAddresses, args.SlirpDNS)
	} else {
		args.DNSAddresses, err = osutil.DNSAddresses()
		if err != nil {
			return err
		}
	}

	args.CACerts.RemoveDefaults = y.CACertificates.RemoveDefaults

	for _, path := range y.CACertificates.Files {
		expanded, err := localpathutil.Expand(path)
		if err != nil {
			return err
		}

		content, err := os.ReadFile(expanded)
		if err != nil {
			return err
		}

		cert := getCert(string(content))
		args.CACerts.Trusted = append(args.CACerts.Trusted, cert)
	}

	for _, content := range y.CACertificates.Certs {
		cert := getCert(content)
		args.CACerts.Trusted = append(args.CACerts.Trusted, cert)
	}

	args.BootCmds = getBootCmds(y.Provision)

	for _, f := range y.Provision {
		if f.Mode == limayaml.ProvisionModeDependency && *f.SkipDefaultDependencyResolution {
			args.SkipDefaultDependencyResolution = true
		}
	}

	if err := ValidateTemplateArgs(args); err != nil {
		return err
	}

	layout, err := ExecuteTemplate(args)
	if err != nil {
		return err
	}

	for i, f := range y.Provision {
		switch f.Mode {
		case limayaml.ProvisionModeSystem, limayaml.ProvisionModeUser, limayaml.ProvisionModeDependency:
			layout = append(layout, iso9660util.Entry{
				Path:   fmt.Sprintf("provision.%s/%08d", f.Mode, i),
				Reader: strings.NewReader(f.Script),
			})
		case limayaml.ProvisionModeBoot:
			continue
		case limayaml.ProvisionModeAnsible:
			continue
		default:
			return fmt.Errorf("unknown provision mode %q", f.Mode)
		}
	}

	guestAgentBinary, err := usrlocalsharelima.GuestAgentBinary(*y.OS, *y.Arch)
	if err != nil {
		return err
	}
	guestAgent, err := os.Open(guestAgentBinary)
	if err != nil {
		return err
	}
	defer guestAgent.Close()
	layout = append(layout, iso9660util.Entry{
		Path:   "lima-guestagent",
		Reader: guestAgent,
	})

	if nerdctlArchive != "" {
		nftgzR, err := os.Open(nerdctlArchive)
		if err != nil {
			return err
		}
		defer nftgzR.Close()
		layout = append(layout, iso9660util.Entry{
			// ISO9660 requires len(Path) <= 30
			Path:   "nerdctl-full.tgz",
			Reader: nftgzR,
		})
	}

	if args.VMType == limayaml.WSL2 {
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

func getBootCmds(p []limayaml.Provision) []BootCmds {
	var bootCmds []BootCmds
	for _, f := range p {
		if f.Mode == limayaml.ProvisionModeBoot {
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
