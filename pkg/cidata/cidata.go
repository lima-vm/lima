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
	NerdctlVersion = "0.11.2"
)

var (
	NerdctlFullDigests = map[limayaml.Arch]digest.Digest{
		limayaml.X8664:   "sha256:27dbb238f9eb248ca68f11b412670db51db84905e3583834400305b2149915f2",
		limayaml.AARCH64: "sha256:fe6322a88cb15d8a502e649827e3d1570210bb038b7a4a52820bce0fec86a637",
	}
)

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
		Name:         name,
		User:         u.Username,
		UID:          uid,
		Containerd:   Containerd{System: *y.Containerd.System, User: *y.Containerd.User},
		SlirpNICName: qemu.SlirpNICName,
		SlirpGateway: qemu.SlirpGateway,
		SlirpDNS:     qemu.SlirpDNS,
		Env:          y.Env,
	}

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

	if len(y.DNS) > 0 {
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
