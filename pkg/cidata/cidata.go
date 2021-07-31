package cidata

import (
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AkihiroSuda/lima/pkg/downloader"
	"github.com/AkihiroSuda/lima/pkg/iso9660util"
	"github.com/AkihiroSuda/lima/pkg/limayaml"
	"github.com/AkihiroSuda/lima/pkg/localpathutil"
	"github.com/AkihiroSuda/lima/pkg/qemu"
	"github.com/AkihiroSuda/lima/pkg/sshutil"
	"github.com/AkihiroSuda/lima/pkg/store/filenames"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	NerdctlVersion = "0.11.0"
)

var (
	NerdctlFullDigests = map[limayaml.Arch]digest.Digest{
		limayaml.X8664:   "sha256:a491a3129beddf2feb41a8ea9dcd0d3e4c9cb36c3a21046675e425f4b3daa5a9",
		limayaml.AARCH64: "sha256:865d8648e2378e10dc2311f5a373fd0b85cbb6c03d2adafb1ca752d0e1267649",
	}
)

func GenerateISO9660(instDir, name string, y *limayaml.LimaYAML) error {
	if err := limayaml.ValidateRaw(*y); err != nil {
		return err
	}
	u, err := user.Current()
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}
	args := TemplateArgs{
		Name:       name,
		User:       u.Username,
		UID:        uid,
		Containerd: Containerd{System: *y.Containerd.System, User: *y.Containerd.User},
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

	args.MACAddresses = append(args.MACAddresses, qemu.SlirpMACAddress)
	if y.Network.VDE.URL != "" {
		args.MACAddresses = append(args.MACAddresses, qemu.MACAddress(instDir, y))
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
			return errors.Errorf("unknown provision mode %q", f.Mode)
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
			return errors.Errorf("unexpected arch %q", y.Arch)
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
			return errors.Wrapf(err, "failed to download %q", nftgzURL)
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

	return nil, errors.Errorf("failed to find \"lima-guestagent.Linux-%s\" binary for %q, attempted %v",
		arch, self, candidates)
}
