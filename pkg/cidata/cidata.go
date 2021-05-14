package cidata

import (
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/AkihiroSuda/lima/pkg/iso9660util"
	"github.com/AkihiroSuda/lima/pkg/limayaml"
	"github.com/AkihiroSuda/lima/pkg/localpathutil"
	"github.com/AkihiroSuda/lima/pkg/sshutil"
	"github.com/pkg/errors"
)

func GenerateISO9660(name string, y *limayaml.LimaYAML) (*iso9660util.ISO9660, error) {
	if err := limayaml.ValidateRaw(*y); err != nil {
		return nil, err
	}
	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, err
	}
	args := TemplateArgs{
		Name: name,
		User: u.Username,
		UID:  uid,
	}
	for _, f := range sshutil.DefaultPubKeys() {
		args.SSHPubKeys = append(args.SSHPubKeys, f.Content)
	}

	for _, f := range y.Mounts {
		expanded, err := localpathutil.Expand(f.Location)
		if err != nil {
			return nil, err
		}
		args.Mounts = append(args.Mounts, expanded)
	}

	if err := ValidateTemplateArgs(args); err != nil {
		return nil, err
	}

	userData, err := GenerateUserData(args)
	if err != nil {
		return nil, err
	}

	metaData, err := GenerateMetaData(args)
	if err != nil {
		return nil, err
	}

	guestAgentBinaryPath, err := GuestAgentBinaryPath(y.Arch)
	if err != nil {
		return nil, err
	}

	iso9660 := &iso9660util.ISO9660{
		Name: "cidata",
		FilesFromContent: map[string]string{
			"user-data": string(userData),
			"meta-data": string(metaData),
		},
		FilesFromHostFilePath: map[string]string{
			"lima-guestagent": guestAgentBinaryPath,
		},
	}

	return iso9660, nil
}

func GuestAgentBinaryPath(arch string) (string, error) {
	if arch == "" {
		return "", errors.New("arch must be set")
	}
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	selfSt, err := os.Stat(self)
	if err != nil {
		return "", err
	}
	if selfSt.Mode()&fs.ModeSymlink != 0 {
		self, err = os.Readlink(self)
		if err != nil {
			return "", err
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
	for _, f := range candidates {
		if _, err := os.Stat(f); err == nil {
			return f, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}

	return "", errors.Errorf("failed to find \"lima-guestagent.Linux-%s\" binary for %q, attempted %v",
		arch, self, candidates)
}
