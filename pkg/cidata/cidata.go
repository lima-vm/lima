package cidata

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lima-vm/lima/pkg/downloader"
	"github.com/lima-vm/lima/pkg/iso9660util"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/localpathutil"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/qemu/const"
	"github.com/lima-vm/lima/pkg/sshutil"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

const (
	NerdctlVersion = "0.12.0"
)

var (
	NerdctlFullDigests = map[limayaml.Arch]digest.Digest{
		limayaml.X8664:   "sha256:7789800cfdd19fa9eccadb5e4a911e4ba759799ad9ec0b7929c983b9d149bc98",
		limayaml.AARCH64: "sha256:ebb05e22ac6a3c25ac88ca4f747feed89bfae8e447a626d0fedf3b4f40ac3303",
	}
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
	for _, name := range append(lowerVars, upperVars...) {
		if value, ok := os.LookupEnv(name); ok {
			if _, ok := env[name]; ok && value != env[name] {
				logrus.Infof("Overriding %q value %q with %q from limactl process environment",
					name, env[name], value)
			}
			env[name] = value
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

func GenerateISO9660(instDir, name string, y *limayaml.LimaYAML) error {
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
		Name:            name,
		User:            u.Username,
		UID:             uid,
		Containerd:      Containerd{System: *y.Containerd.System, User: *y.Containerd.User},
		SlirpNICName:    qemu.SlirpNICName,
		SlirpGateway:    qemu.SlirpGateway,
		SlirpDNS:        qemu.SlirpDNS,
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
	if *y.UseHostResolver {
		args.UDPDNSLocalPort = y.SSH.LocalPort
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

	if guestAgentBinary, err := GuestAgentBinary(y.Arch); err != nil {
		return err
	} else {
		defer guestAgentBinary.Close()
		layout = append(layout, iso9660util.Entry{
			Path:   "lima-guestagent",
			Reader: guestAgentBinary,
		})
	}

	if args.Containerd.System || args.Containerd.User {
		var nftgzBase string
		switch y.Arch {
		case limayaml.X8664:
			nftgzBase = fmt.Sprintf("nerdctl-full-%s-linux-amd64.tar.gz", NerdctlVersion)
		case limayaml.AARCH64:
			nftgzBase = fmt.Sprintf("nerdctl-full-%s-linux-arm64.tar.gz", NerdctlVersion)
		default:
			return fmt.Errorf("unexpected arch %q", y.Arch)
		}
		td, err := ioutil.TempDir("", "lima-download-nerdctl")
		if err != nil {
			return err
		}
		defer os.RemoveAll(td)
		nftgzLocal := filepath.Join(td, nftgzBase)
		nftgzURL := fmt.Sprintf("https://github.com/containerd/nerdctl/releases/download/v%s/%s",
			NerdctlVersion, nftgzBase)
		nftgzDigest := NerdctlFullDigests[y.Arch]
		logrus.Infof("Downloading %q (%s)", nftgzURL, nftgzDigest)
		res, err := downloader.Download(nftgzLocal, nftgzURL, downloader.WithCache(), downloader.WithExpectedDigest(nftgzDigest))
		if err != nil {
			return fmt.Errorf("failed to download %q: %w", nftgzURL, err)
		}
		logrus.Debugf("res.ValidatedDigest=%v", res.ValidatedDigest)
		switch res.Status {
		case downloader.StatusDownloaded:
			logrus.Infof("Downloaded %q", nftgzBase)
		case downloader.StatusUsedCache:
			logrus.Infof("Using cache %q", res.CachePath)
		default:
			logrus.Warnf("Unexpected result from downloader.Download(): %+v", res)
		}

		nftgzR, err := os.Open(nftgzLocal)
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
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	selfSt, err := os.Stat(self)
	if err != nil {
		return nil, err
	}
	if selfSt.Mode()&fs.ModeSymlink != 0 {
		self, err = os.Readlink(self)
		if err != nil {
			return nil, err
		}
	}

	// self:  /usr/local/bin/limactl
	selfDir := filepath.Dir(self)
	selfDirDir := filepath.Dir(selfDir)
	candidates := []string{
		// candidate 0:
		// - self:  /Applications/Lima.app/Contents/MacOS/limactl
		// - agent: /Applications/Lima.app/Contents/MacOS/lima-guestagent.Linux-x86_64
		filepath.Join(selfDir, "lima-guestagent.Linux-"+arch),
		// candidate 1:
		// - self:  /usr/local/bin/limactl
		// - agent: /usr/local/share/lima/lima-guestagent.Linux-x86_64
		filepath.Join(selfDirDir, "share/lima/lima-guestagent.Linux-"+arch),
		// TODO: support custom path
	}
	for _, candidate := range candidates {
		if f, err := os.Open(candidate); err == nil {
			return f, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("failed to find \"lima-guestagent.Linux-%s\" binary for %q, attempted %v",
		arch, self, candidates)
}
