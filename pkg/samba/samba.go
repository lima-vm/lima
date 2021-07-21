package samba

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/AkihiroSuda/lima/pkg/limayaml"
	"github.com/AkihiroSuda/lima/pkg/localpathutil"
	"github.com/AkihiroSuda/lima/pkg/samba/smbpasswd"
	"github.com/AkihiroSuda/lima/pkg/store/filenames"
	"github.com/AkihiroSuda/lima/pkg/templateutil"
)

func SMBD() (string, error) {
	smbd := "smbd"
	if runtime.GOOS == "darwin" {
		// We have to use Samba's smbd, not Apple's smbd.
		smbd = "/usr/local/sbin/samba-dot-org-smbd"
	}
	if v := os.Getenv("SMBD"); v != "" {
		smbd = v
	}
	exe, err := exec.LookPath(smbd)
	if err != nil && runtime.GOOS == "darwin" {
		err = fmt.Errorf("%w (hint: `brew install samba`)", err)
	}
	return exe, err
}

type Samba struct {
	SMBD     string
	SMBConf  string
	StateDir string
}

func New(instDir string, mounts []limayaml.Mount) (*Samba, error) {
	smbd, err := SMBD()
	if err != nil {
		return nil, err
	}

	plainPassword := "PLAIN-PASSWORD-FIXME-THIS-SHOULD-BE-AUTO-GENERATED"

	smbPasswd, err := smbpasswd.SMBPasswdForCurrentUser(plainPassword)
	if err != nil {
		return nil, err
	}

	sambaD := filepath.Join(instDir, filenames.Samba)
	if err := os.RemoveAll(sambaD); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(sambaD, 0700); err != nil {
		return nil, err
	}
	sambaState := filepath.Join(instDir, filenames.SambaState)
	if err := os.MkdirAll(sambaState, 0700); err != nil {
		return nil, err
	}
	sambaStateSMBPasswd := filepath.Join(sambaState, "smbpasswd")
	if err := os.WriteFile(sambaStateSMBPasswd, []byte(smbPasswd), 0600); err != nil {
		return nil, err
	}

	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	sambaCredentialsContent := fmt.Sprintf("username=%s\npassword=%s\n", u.Username, plainPassword)
	sambaCredentials := filepath.Join(instDir, filenames.SambaCredentials)
	if err := os.WriteFile(sambaCredentials, []byte(sambaCredentialsContent), 0600); err != nil {
		return nil, err
	}

	smbConf := filepath.Join(instDir, filenames.SambaSMBConf)

	for i := range mounts {
		expanded, err := localpathutil.Expand(mounts[i].Location)
		if err != nil {
			return nil, err
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
		return nil, err
	}
	if err := os.WriteFile(smbConf, smbConfContent, 0600); err != nil {
		return nil, err
	}

	samba := &Samba{
		SMBD:     smbd,
		SMBConf:  smbConf,
		StateDir: sambaState,
	}
	return samba, nil
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
# TODO: remove map to guest
map to guest=Bad User
load printers=no
printing=bsd
disable spoolss=yes
name resolve order=lmhosts host

{{- range $i, $val := .Mounts}}
[lima-{{$i}}]
path={{$val.Location}}
valid users={{$.Username}}
force user={{$.Username}}
{{- if $val.Writable }}
read only=no
{{- end }}
{{- end }}
`
