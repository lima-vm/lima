// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package cidata

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/sethvargo/go-password/password"

	"github.com/lima-vm/lima/v2/pkg/identifiers"
	"github.com/lima-vm/lima/v2/pkg/iso9660util"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/textutil"
)

//go:embed cidata.TEMPLATE.d
var templateFS embed.FS

const templateFSRoot = "cidata.TEMPLATE.d"

//go:embed wincidata.TEMPLATE.d
var windowsTemplateFS embed.FS

const windowsTemplateFSRoot = "wincidata.TEMPLATE.d"

// This is for checking whether Windows OS is 11 or server 2025.
// For example, Windows 11 x86-64's label is CCCOMA_X64FRE_EN-US_DV9.
const windowsClientISOLabelPrefix = "CCCOMA_"

type CACerts struct {
	RemoveDefaults *bool
	Trusted        []Cert
}

type Cert struct {
	Lines []string
}

type Containerd struct {
	System  bool
	User    bool
	Archive string
}
type Network struct {
	MACAddress string
	Interface  string
	Metric     uint32
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

type DataFile struct {
	FileName    string
	Overwrite   string
	Owner       string
	Path        string
	Permissions string
}

type YQProvision struct {
	FileName    string
	Format      string
	Owner       string
	Path        string
	Permissions string
}

type Disk struct {
	Name   string
	Device string
	Format bool
	FSType string
	FSArgs []string
}
type TemplateArgs struct {
	Debug                           bool
	OS                              limatype.OS
	Arch                            limatype.Arch
	Name                            string // instance name
	Hostname                        string // instance hostname
	IID                             string // instance id
	User                            string // user name
	Comment                         string // user information
	Home                            string // home directory
	Shell                           string // login shell
	UID                             uint32
	PasswordlessSudo                bool
	SSHPubKeys                      []string
	Mounts                          []Mount
	MountType                       string
	Disks                           []Disk
	GuestInstallPrefix              string
	UpgradePackages                 bool
	Containerd                      Containerd
	Networks                        []Network
	SlirpNICName                    string
	SlirpGateway                    string
	SlirpDNS                        string
	SlirpIPAddress                  string
	UDPDNSLocalPort                 int
	TCPDNSLocalPort                 int
	Env                             map[string]string
	Param                           map[string]string
	BootScripts                     bool
	DataFiles                       []DataFile
	YQProvisions                    []YQProvision
	DNSAddresses                    []string
	CACerts                         CACerts
	HostHomeMountPoint              string
	BootCmds                        []BootCmds
	RosettaEnabled                  bool
	RosettaBinFmt                   bool
	SkipDefaultDependencyResolution bool
	VMType                          string
	VSockPort                       int
	VirtioPort                      string
	Plain                           bool
	TimeZone                        string
	NoCloudInit                     bool
	WindowsInitialPassword          string
	LegacyBIOS                      bool
	IsWindowsServer                 bool
	TPM                             bool
}

func (t *TemplateArgs) generateWindowsInitialPassword() error {
	const pwLen = 16
	// Avoid special characters to minimize potential keyboard layout issue.
	pw, err := password.Generate(pwLen, pwLen/4, 0, false, false)
	if err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}

	t.WindowsInitialPassword = pw
	return nil
}

// checkWindowsVersion checks if a guest VM is Windows 11 (true) or Windows server 2025 (false).
func (t *TemplateArgs) checkWindowsVersion(instDir string) error {
	imagePath := filepath.Join(instDir, filenames.ISO)
	label, err := iso9660util.Label(imagePath)
	if err != nil {
		return fmt.Errorf("failed to get ISO label: %w", err)
	}

	t.IsWindowsServer = !strings.HasPrefix(label, windowsClientISOLabelPrefix)

	return nil
}

func ValidateTemplateArgs(args *TemplateArgs) error {
	if err := identifiers.Validate(args.Name); err != nil {
		return err
	}
	// args.User is intentionally not validated here; the user can override with any name they want
	// limayaml.FillDefault will validate the default (local) username, but not an explicit setting
	if args.User == "root" {
		return errors.New("field User must not be `root`")
	}
	if args.UID == 0 {
		return errors.New("field UID must not be 0")
	}
	if args.Home == "" {
		return errors.New("field Home must be set")
	}
	if args.Shell == "" {
		return errors.New("field Shell must be set")
	}
	if len(args.SSHPubKeys) == 0 {
		return errors.New("field SSHPubKeys must be set")
	}
	for i, m := range args.Mounts {
		f := m.MountPoint
		if !path.IsAbs(f) {
			return fmt.Errorf("field mounts[%d] must be absolute, got %#q", i, f)
		}
	}
	return nil
}

func ExecuteTemplateCloudConfig(args *TemplateArgs) ([]byte, error) {
	if err := ValidateTemplateArgs(args); err != nil {
		return nil, err
	}

	userData, err := templateFS.ReadFile(path.Join(templateFSRoot, "user-data"))
	if err != nil {
		return nil, err
	}

	cloudConfigYaml := string(userData)
	return textutil.ExecuteTemplate(cloudConfigYaml, args)
}

func ExecuteTemplateCIDataISO(args *TemplateArgs) ([]iso9660util.Entry, error) {
	if err := ValidateTemplateArgs(args); err != nil {
		return nil, err
	}

	fsys, err := fs.Sub(templateFS, templateFSRoot)
	if err != nil {
		return nil, err
	}

	var layout []iso9660util.Entry
	walkFn := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("got non-regular file %#q", path)
		}
		templateB, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		b, err := textutil.ExecuteTemplate(string(templateB), args)
		if err != nil {
			return err
		}
		layout = append(layout, iso9660util.Entry{
			Path:   path,
			Reader: bytes.NewReader(b),
		})
		return nil
	}

	if err := fs.WalkDir(fsys, ".", walkFn); err != nil {
		return nil, err
	}

	return layout, nil
}

func ExecuteTemplateWindowsISO(args *TemplateArgs) ([]iso9660util.Entry, error) {
	fs := windowsTemplateFS
	root := windowsTemplateFSRoot

	// Execute template for autounattend.xml
	xmlTemplate, err := fs.ReadFile(path.Join(root, "autounattend.xml"))
	if err != nil {
		return nil, err
	}

	xmlfile, err := textutil.ExecuteTemplate(string(xmlTemplate), args)
	if err != nil {
		return nil, fmt.Errorf("failed to render autounattend.xml: %w", err)
	}

	// Execute template for powershell script file
	ps1Template, err := fs.ReadFile(path.Join(root, "first_logon.ps1"))
	if err != nil {
		return nil, err
	}

	ps1file, err := textutil.ExecuteTemplate(string(ps1Template), args)
	if err != nil {
		return nil, fmt.Errorf("failed to render ps1 file: %w", err)
	}

	layout := []iso9660util.Entry{
		{
			Path:   "autounattend.xml",
			Reader: bytes.NewReader(xmlfile),
		},
		{
			Path:   "first_logon.ps1",
			Reader: bytes.NewReader(ps1file),
		},
	}

	return layout, nil
}
