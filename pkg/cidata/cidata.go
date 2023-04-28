package cidata

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lima-vm/lima/pkg/networks"

	"github.com/docker/go-units"
	"github.com/lima-vm/lima/pkg/iso9660util"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/localpathutil"
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

func setupEnv(y *limayaml.LimaYAML) (map[string]string, error) {
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
	for _, name := range append(lowerVars, upperVars...) {
		value, ok := env[name]
		if ok && !strings.EqualFold(name, "no_proxy") {
			u, err := url.Parse(value)
			if err != nil {
				logrus.Warnf("Ignoring invalid proxy %q=%v: %s", name, value, err)
				continue
			}

			for _, ip := range netLookupIP(u.Hostname()) {
				if ip.IsLoopback() {
					newHost := networks.SlirpGateway
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

func GenerateISO9660(instDir, name string, y *limayaml.LimaYAML, udpDNSLocalPort, tcpDNSLocalPort int, nerdctlArchive string) error {
	if err := limayaml.Validate(*y, false); err != nil {
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
		GuestInstallPrefix: *y.GuestInstallPrefix,
		Containerd:         Containerd{System: *y.Containerd.System, User: *y.Containerd.User},
		SlirpNICName:       networks.SlirpNICName,
		SlirpGateway:       networks.SlirpGateway,
		SlirpDNS:           networks.SlirpDNS,
		SlirpIPAddress:     networks.SlirpIPAddress,
		RosettaEnabled:     *y.Rosetta.Enabled,
		RosettaBinFmt:      *y.Rosetta.BinFmt,
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
		args.Disks = append(args.Disks, Disk{
			Name:   d.Name,
			Device: diskDeviceNameFromOrder(i),
		})
	}

	slirpMACAddress := limayaml.MACAddress(instDir)
	args.Networks = append(args.Networks, Network{MACAddress: slirpMACAddress, Interface: networks.SlirpNICName})
	firstUsernetIndex := limayaml.FirstUsernetIndex(y)
	for i, nw := range y.Networks {
		if i == firstUsernetIndex {
			continue
		}
		args.Networks = append(args.Networks, Network{MACAddress: nw.MACAddress, Interface: nw.Interface})
	}

	args.Env, err = setupEnv(y)
	if err != nil {
		return err
	}
	if *y.HostResolver.Enabled {
		args.UDPDNSLocalPort = udpDNSLocalPort
		args.TCPDNSLocalPort = tcpDNSLocalPort
		args.DNSAddresses = append(args.DNSAddresses, networks.SlirpDNS)
	} else if len(y.DNS) > 0 {
		for _, addr := range y.DNS {
			args.DNSAddresses = append(args.DNSAddresses, addr.String())
		}
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
		default:
			return fmt.Errorf("unknown provision mode %q", f.Mode)
		}
	}

	guestAgentBinary, err := GuestAgentBinary(*y.Arch)
	if err != nil {
		return err
	}
	defer guestAgentBinary.Close()
	layout = append(layout, iso9660util.Entry{
		Path:   "lima-guestagent",
		Reader: guestAgentBinary,
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

	return iso9660util.Write(filepath.Join(instDir, filenames.CIDataISO), "cidata", layout)
}

func GuestAgentBinary(arch string) (io.ReadCloser, error) {
	if arch == "" {
		return nil, errors.New("arch must be set")
	}
	dir, err := usrlocalsharelima.Dir()
	if err != nil {
		return nil, err
	}
	gaPath := filepath.Join(dir, "lima-guestagent.Linux-"+arch)
	return os.Open(gaPath)
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
