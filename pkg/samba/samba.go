package samba

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/localpathutil"
	"github.com/lima-vm/lima/pkg/passwdgen"
	"github.com/lima-vm/lima/pkg/samba/smbpasswd"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/lima-vm/lima/pkg/templateutil"
)

func SMBDBinary() (string, error) {
	var candidates []string
	if v := os.Getenv("SMBD"); v != "" {
		candidates = []string{v}
	} else {
		smbdBinary := "smbd"
		if runtime.GOOS == "darwin" {
			// We have to use Samba's smbd, not Apple's smbd.
			smbdBinary = "samba-dot-org-smbd"
		}
		self, err := os.Executable()
		if err != nil {
			return "", err
		}
		binDir := filepath.Dir(self)
		prefixDir := filepath.Dir(binDir)
		sbinDir := filepath.Join(prefixDir, "sbin")
		candidates = []string{
			filepath.Join(sbinDir, smbdBinary),
			smbdBinary,
			filepath.Join("/usr/local/sbin", smbdBinary),
			filepath.Join("/usr/sbin", smbdBinary),
		}
	}

	for _, f := range candidates {
		exe, err := exec.LookPath(f)
		if err == nil {
			return exe, nil
		}
	}
	return "", fmt.Errorf("could not find samba daemon process (hint: `brew install samba`)")
}

// InitConfig initializes the samba config.
// Password is recreated every time.
func InitConfig(instDir string, mounts []limayaml.Mount) error {
	if _, err := SMBDBinary(); err != nil {
		return fmt.Errorf("samba binary could not be found: %w", err)
	}

	sambaD := filepath.Join(instDir, filenames.Samba)
	if err := os.RemoveAll(sambaD); err != nil {
		return err
	}
	if err := os.MkdirAll(sambaD, 0700); err != nil {
		return err
	}
	sambaState := filepath.Join(instDir, filenames.SambaState)
	sambaStateSMBPasswd := filepath.Join(sambaState, "smbpasswd")
	sambaCredentials := filepath.Join(instDir, filenames.SambaCredentials)
	u, err := user.Current()
	if err != nil {
		return err
	}
	plainPassword := passwdgen.GeneratePassword(64)
	smbPasswd, err := smbpasswd.SMBPasswdForCurrentUser(plainPassword)
	if err != nil {
		return err
	}
	sambaCredentialsContent := fmt.Sprintf("username=%s\npassword=%s\n", u.Username, plainPassword)

	smbConf := filepath.Join(instDir, filenames.SambaSMBConf)

	for i := range mounts {
		expanded, err := localpathutil.Expand(mounts[i].Location)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(expanded, 0700); err != nil {
			return err
		}
		mounts[i].Location = expanded
	}

	smbConfArgs := &smbConfTmplArgs{
		StateDir:      sambaState,
		SMBPasswdFile: sambaStateSMBPasswd,
		Username:      u.Username,
		Mounts:        mounts,
	}
	smbConfContent, err := templateutil.Execute(smbConfTmpl, smbConfArgs)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(sambaState, 0700); err != nil {
		return err
	}

	if err := os.WriteFile(sambaStateSMBPasswd, []byte(smbPasswd), 0600); err != nil {
		return err
	}

	if err := os.WriteFile(sambaCredentials, []byte(sambaCredentialsContent), 0600); err != nil {
		return err
	}

	if err := os.WriteFile(smbConf, smbConfContent, 0600); err != nil {
		return err
	}
	return nil
}

type smbConfTmplArgs struct {
	StateDir      string
	SMBPasswdFile string
	Username      string
	Mounts        []limayaml.Mount
}

const smbConfTmpl = `
# Automatically generated. DO NOT EDIT.
[global]
private dir={{.StateDir}}
# This "interfaces" option is dummy. We speak Samba over Stdio. We do not even use loopback interfaces.
interfaces=127.0.0.1
bind interfaces only=yes
pid directory={{.StateDir}}
lock directory={{.StateDir}}
state directory={{.StateDir}}
cache directory={{.StateDir}}
ncalrpc dir={{.StateDir}}/ncalrpc
log file={{.StateDir}}/log.smbd
passdb backend=smbpasswd
smb passwd file={{.SMBPasswdFile}}
security=user
load printers=no
printing=bsd
disable spoolss=yes
name resolve order=lmhosts host
unix charset = UTF8

# Enable SMB1, to support unix extensions
# FIXME: remove this when Samba supports unix extensions for SMB3.1.1
# https://bugs.launchpad.net/ubuntu/+source/samba/+bug/1883234
unix extensions=yes
server min protocol = NT1
client min protocol = NT1

{{- range $i, $val := .Mounts}}
[lima-{{$i}}]
path={{$val.Location}}
valid users={{$.Username}}
{{- if $val.Writable }}
read only=no
{{- end }}
{{- end }}
`
