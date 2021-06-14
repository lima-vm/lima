package cidata

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/AkihiroSuda/lima/pkg/downloader"
	"github.com/AkihiroSuda/lima/pkg/iso9660util"
	"github.com/AkihiroSuda/lima/pkg/limayaml"
	"github.com/AkihiroSuda/lima/pkg/localpathutil"
	"github.com/AkihiroSuda/lima/pkg/sshutil"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const NerdctlVersion = "0.8.3"

func GenerateISO9660(isoPath, name string, y *limayaml.LimaYAML) error {
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
		Provision:  y.Provision,
		Containerd: Containerd{System: *y.Containerd.System, User: *y.Containerd.User},
	}

	pubKeys := sshutil.DefaultPubKeys()
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

	if err := ValidateTemplateArgs(args); err != nil {
		return err
	}

	var layout []iso9660util.Entry

	if userData, err := GenerateUserData(args); err != nil {
		return err
	} else {
		layout = append(layout, iso9660util.Entry{
			Path:   "user-data",
			Reader: bytes.NewReader(userData),
		})
	}

	if metaData, err := GenerateMetaData(args); err != nil {
		return err
	} else {
		layout = append(layout, iso9660util.Entry{
			Path:   "meta-data",
			Reader: bytes.NewReader(metaData),
		})
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
		logrus.Infof("Downloading %q", nftgzURL)
		res, err := downloader.Download(nftgzLocal, nftgzURL, downloader.WithCache())
		if err != nil {
			return errors.Wrapf(err, "failed to download %q", nftgzURL)
		}
		switch res.Status {
		case downloader.StatusDownloaded:
			logrus.Infof("Downloaded %q", nftgzBase)
		case downloader.StatusUsedCache:
			logrus.Infof("Using cache %q", res.CachePath)
		default:
			logrus.Warnf("Unexpected result from downloader.Download(): %+v", res)
		}
		// TODO: verify sha256
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

	return iso9660util.Write(isoPath, "cidata", layout)
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
