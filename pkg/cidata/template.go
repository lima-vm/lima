// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package cidata

import (
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"math/big"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lima-vm/lima/v2/pkg/identifiers"
	"github.com/lima-vm/lima/v2/pkg/iso9660util"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
	"github.com/lima-vm/lima/v2/pkg/textutil"
	"github.com/sirupsen/logrus"
)

//go:embed cidata.TEMPLATE.d
var templateFS embed.FS

const templateFSRoot = "cidata.TEMPLATE.d"

//go:embed windows.TEMPLATE.d
var windowsTemplateFS embed.FS

const windowsTemplateFSRoot = "windows.TEMPLATE.d"

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
	Name                            string // instance name
	Hostname                        string // instance hostname
	IID                             string // instance id
	User                            string // user name
	Comment                         string // user information
	Home                            string // home directory
	Shell                           string // login shell
	UID                             uint32
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
	WindowsUser                     string
	WindowsPassword                 string
	WindowsImageIndex               string
	WindowsDriverPaths              []string
}

func ValidateTemplateArgs(args *TemplateArgs) error {
	if err := identifiers.Validate(args.Name); err != nil {
		return err
	}
	// args.User is intentionally not validated here; the user can override with any name they want
	// limayaml.FillDefault will validate the default (local) username, but not an explicit setting
	if args.User == "root" {
		return errors.New("field User must not be \"root\"")
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
			return fmt.Errorf("field mounts[%d] must be absolute, got %q", i, f)
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
	return executeTemplateCIDataFS(fsys, args, func(p string) string { return p })
}

func ExecuteTemplateWindowsCIDataISO(args *TemplateArgs) ([]iso9660util.Entry, error) {
	fsys, err := fs.Sub(windowsTemplateFS, windowsTemplateFSRoot)
	if err != nil {
		return nil, err
	}
	return executeTemplateCIDataFS(fsys, args, windowsTemplatePath)
}

func executeTemplateCIDataFS(fsys fs.FS, args *TemplateArgs, mapPath func(string) string) ([]iso9660util.Entry, error) {
	var layout []iso9660util.Entry
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
			Path:   mapPath(p),
			Reader: bytes.NewReader(b),
		})
		return nil
	}
	if err := fs.WalkDir(fsys, ".", walkFn); err != nil {
		return nil, err
	}
	return layout, nil
}

func windowsTemplatePath(p string) string {
	switch {
	case strings.HasPrefix(p, "oem/SystemRoot/"):
		return path.Join("$OEM$", "$$", strings.TrimPrefix(p, "oem/SystemRoot/"))
	case strings.HasPrefix(p, "oem/SystemDrive/"):
		return path.Join("$OEM$", "$1", strings.TrimPrefix(p, "oem/SystemDrive/"))
	default:
		return p
	}
}

func TemplateArgsForWindows(ctx context.Context, instDir string, instConfig *limatype.LimaYAML) (*TemplateArgs, error) {
	loadDotSSHPubKeys := false
	if instConfig.SSH.LoadDotSSHPubKeys != nil {
		loadDotSSHPubKeys = *instConfig.SSH.LoadDotSSHPubKeys
	}
	pubKeys, err := sshutil.DefaultPubKeys(ctx, loadDotSSHPubKeys)
	if err != nil {
		return nil, err
	}
	if len(pubKeys) == 0 {
		return nil, errors.New("no SSH key was found, run `ssh-keygen`")
	}
	sshPubKeys := make([]string, 0, len(pubKeys))
	for _, f := range pubKeys {
		sshPubKeys = append(sshPubKeys, f.Content)
	}
	return templateArgsForWindows(instDir, instConfig, sshPubKeys)
}

func templateArgsForWindows(instDir string, instConfig *limatype.LimaYAML, sshPubKeys []string) (*TemplateArgs, error) {
	driverArch, err := windowsVirtioArch(*instConfig.Arch)
	if err != nil {
		return nil, err
	}
	windowsPassword, err := windowsUserPassword(instDir, instConfig)
	if err != nil {
		return nil, err
	}
	args := &TemplateArgs{
		WindowsUser:        "lima",
		WindowsPassword:    windowsPassword,
		WindowsImageIndex:  "1",
		WindowsDriverPaths: windowsVirtioDriverPaths(windowsVirtioVersion(instConfig), driverArch),
		SSHPubKeys:         sshPubKeys,
	}
	if instConfig.User.Name != nil && *instConfig.User.Name != "" {
		args.WindowsUser = *instConfig.User.Name
	}
	args.WindowsUser = html.EscapeString(args.WindowsUser)
	args.WindowsPassword = html.EscapeString(args.WindowsPassword)
	args.WindowsImageIndex = html.EscapeString(args.WindowsImageIndex)
	for i := range args.WindowsDriverPaths {
		args.WindowsDriverPaths[i] = html.EscapeString(args.WindowsDriverPaths[i])
	}
	return args, nil
}

func windowsUserPassword(instDir string, instConfig *limatype.LimaYAML) (string, error) {
	if instConfig.User.Password != nil && *instConfig.User.Password != "" {
		return *instConfig.User.Password, nil
	}
	passwordPath := filepath.Join(instDir, filenames.WindowsUserPassword)
	if b, err := os.ReadFile(passwordPath); err == nil {
		if password := strings.TrimSpace(string(b)); password != "" {
			if isAlphaNumeric(password) {
				return password, nil
			}
			logrus.Infof("Replacing generated Windows user password in %q because it contains non-alphanumeric characters", passwordPath)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	generated, err := generateAlphaNumericPassword(24)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(passwordPath, []byte(generated+"\n"), 0o600); err != nil {
		return "", err
	}
	logrus.Infof("Generated Windows user password and stored it in %q", passwordPath)
	return generated, nil
}

func generateAlphaNumericPassword(length int) (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		b[i] = alphabet[n.Int64()]
	}
	return string(b), nil
}

func isAlphaNumeric(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			continue
		}
		return false
	}
	return s != ""
}
