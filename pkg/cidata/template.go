package cidata

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path"

	"github.com/lima-vm/lima/pkg/iso9660util"

	"github.com/containerd/containerd/identifiers"
	"github.com/lima-vm/lima/pkg/textutil"
	"github.com/sirupsen/logrus"
)

//go:embed cidata.TEMPLATE.d
var templateFS embed.FS

const templateFSRoot = "cidata.TEMPLATE.d"

type CACerts struct {
	RemoveDefaults *bool
	Trusted        []Cert
}

type Cert struct {
	Lines []string
}

type Containerd struct {
	System bool
	User   bool
}
type Network struct {
	MACAddress string
	Interface  string
}
type Mount struct {
	Tag        string
	MountPoint string // abs path, accessible by the User
	Type       string
	Options    string
}
type BootCmds struct {
	Lines []string
}
type Disk struct {
	Name   string
	Device string
	Format bool
	FSType string
	FSArgs []string
}
type TemplateArgs struct {
	Name                            string // instance name
	IID                             string // instance id
	User                            string // user name
	Home                            string // home directory
	UID                             int
	SSHPubKeys                      []string
	Mounts                          []Mount
	MountType                       string
	Disks                           []Disk
	GuestInstallPrefix              string
	Containerd                      Containerd
	Networks                        []Network
	SlirpNICName                    string
	SlirpGateway                    string
	SlirpDNS                        string
	SlirpIPAddress                  string
	UDPDNSLocalPort                 int
	TCPDNSLocalPort                 int
	Env                             map[string]string
	DNSAddresses                    []string
	CACerts                         CACerts
	HostHomeMountPoint              string
	BootCmds                        []BootCmds
	RosettaEnabled                  bool
	RosettaBinFmt                   bool
	SkipDefaultDependencyResolution bool
	VMType                          string
	VSockPort                       int
}

func ValidateTemplateArgs(args TemplateArgs) error {
	if err := identifiers.Validate(args.Name); err != nil {
		return err
	}
	if err := identifiers.Validate(args.User); err != nil {
		return err
	}
	if args.User == "root" {
		return errors.New("field User must not be \"root\"")
	}
	if args.UID == 0 {
		return errors.New("field UID must not be 0")
	}
	if args.Home == "" {
		return errors.New("field Home must be set")
	}
	if len(args.SSHPubKeys) == 0 {
		return errors.New("field SSHPubKeys must be set")
	}
	for i, m := range args.Mounts {
		f := m.MountPoint
		if !path.IsAbs(f) {
			return fmt.Errorf("field mounts[%d] must be absolute, got %q", i, f)
		}
	}
	return nil
}

func executeButane(yaml []byte) ([]byte, error) {
	butane, err := exec.LookPath("butane")
	if err != nil {
		logrus.Debug("butane not found in PATH, skipping ignition")
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(butane, "--strict")
	cmd.Stdin = bytes.NewReader(yaml)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logrus.Warnf("%v %s: %s %s", cmd, yaml, err, stderr.String())
		return nil, err
	}
	return stdout.Bytes(), nil
}

func ExecuteTemplate(args TemplateArgs) ([]iso9660util.Entry, []byte, error) {
	if err := ValidateTemplateArgs(args); err != nil {
		return nil, nil, err
	}

	fsys, err := fs.Sub(templateFS, templateFSRoot)
	if err != nil {
		return nil, nil, err
	}

	var layout []iso9660util.Entry
	var ignition []byte
	walkFn := func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("got non-regular file %q", p)
		}
		templateB, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		b, err := textutil.ExecuteTemplate(string(templateB), args)
		if err != nil {
			return err
		}
		layout = append(layout, iso9660util.Entry{
			Path:   p,
			Reader: bytes.NewReader(b),
		})
		if path.Base(p) == "ignition.yaml" {
			ign, err := executeButane([]byte(b))
			if err == nil {
				ignition = ign
			}
		}
		return nil
	}

	if err := fs.WalkDir(fsys, ".", walkFn); err != nil {
		return nil, nil, err
	}

	return layout, ignition, nil
}
