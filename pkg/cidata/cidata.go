package cidata

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lima-vm/lima/pkg/iso9660util"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/localpathutil"
	"github.com/lima-vm/lima/pkg/osutil"
	qemu "github.com/lima-vm/lima/pkg/qemu/const"
	"github.com/lima-vm/lima/pkg/sshutil"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/lima-vm/lima/pkg/usrlocalsharelima"
	"github.com/sirupsen/logrus"
)

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
	// Replace "localhost" in proxy settings with the gateway address
	localhostRegexes := []*regexp.Regexp{
		regexp.MustCompile(`\blocalhost\b`),
		regexp.MustCompile(`\b127.0.0.1\b`),
	}
	for _, name := range append(lowerVars, upperVars...) {
		value, ok := env[name]
		if ok && !strings.EqualFold(name, "no_proxy") {
			for _, re := range localhostRegexes {
				value = re.ReplaceAllString(value, qemu.SlirpGateway)
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
		Name:           name,
		User:           u.Username,
		UID:            uid,
		Containerd:     Containerd{System: *y.Containerd.System, User: *y.Containerd.User},
		SlirpNICName:   qemu.SlirpNICName,
		SlirpGateway:   qemu.SlirpGateway,
		SlirpDNS:       qemu.SlirpDNS,
		SlirpIPAddress: qemu.SlirpIPAddress,
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

	for _, f := range y.Mounts {
		expanded, err := localpathutil.Expand(f.Location)
		if err != nil {
			return err
		}
		args.Mounts = append(args.Mounts, expanded)
	}

	slirpMACAddress := limayaml.MACAddress(instDir)
	args.Networks = append(args.Networks, Network{MACAddress: slirpMACAddress, Interface: qemu.SlirpNICName})
	for _, nw := range y.Networks {
		args.Networks = append(args.Networks, Network{MACAddress: nw.MACAddress, Interface: nw.Interface})
	}

	args.Env, err = setupEnv(y)
	if err != nil {
		return err
	}
	if *y.HostResolver.Enabled {
		args.UDPDNSLocalPort = udpDNSLocalPort
		args.TCPDNSLocalPort = tcpDNSLocalPort
		args.DNSAddresses = append(args.DNSAddresses, qemu.SlirpDNS)
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

	if err := ValidateTemplateArgs(args); err != nil {
		return err
	}

	layout, err := ExecuteTemplate(args)
	if err != nil {
		return err
	}

	for i, f := range y.Provision {
		switch f.Mode {
		case limayaml.ProvisionModeSystem, limayaml.ProvisionModeUser:
			layout = append(layout, iso9660util.Entry{
				Path:   fmt.Sprintf("provision.%s/%08d", f.Mode, i),
				Reader: strings.NewReader(f.Script),
			})
		default:
			return fmt.Errorf("unknown provision mode %q", f.Mode)
		}
	}

	if guestAgentBinary, err := GuestAgentBinary(*y.Arch); err != nil {
		return err
	} else {
		defer guestAgentBinary.Close()
		layout = append(layout, iso9660util.Entry{
			Path:   "lima-guestagent",
			Reader: guestAgentBinary,
		})
	}

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
