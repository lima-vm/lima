// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package toolset

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/instance/hostname"
	"github.com/lima-vm/lima/v2/pkg/mcp/msi"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/store/filenames"
)

func New(inst *store.Instance) (*ToolSet, error) {
	if inst.Status != store.StatusRunning {
		return nil, fmt.Errorf("expected status of instance %q to be %q, got %q",
			inst.Name, store.StatusRunning, inst.Status)
	}
	if len(inst.Config.Mounts) == 0 {
		logrus.Warnf("instance %q has no mount", inst.Name)
	}

	limactl, err := os.Executable()
	if err != nil {
		return nil, err
	}

	sftpClient, sftpCmd, err := newSFTPClient(inst)
	if err != nil {
		return nil, err
	}

	ts := &ToolSet{
		inst:    inst,
		limactl: limactl,
		sftp:    sftpClient,
		sftpCmd: sftpCmd,
	}
	return ts, nil
}

func newSFTPClient(inst *store.Instance) (*sftp.Client, *exec.Cmd, error) {
	ssh0, ssh0Args, err := sshutil.SSHArguments()
	if err != nil {
		return nil, nil, err
	}
	ssh := append([]string{ssh0}, ssh0Args...)
	ssh = append(ssh, "-F",
		filepath.Join(inst.Dir, filenames.SSHConfig),
		hostname.FromInstName(inst.Name))

	cmd := exec.Command(ssh[0], append(ssh[1:], "-s", "sftp")...)
	cmd.Stderr = os.Stderr
	w, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, nil, err
	}
	client, err := sftp.NewClientPipe(r, w)
	if err != nil {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
	return client, cmd, err
}

type ToolSet struct {
	inst    *store.Instance
	limactl string // needed for `limactl shell --workdir`

	sftp    *sftp.Client
	sftpCmd *exec.Cmd
}

func (ts *ToolSet) RegisterServer(server *mcp.Server) error {
	mcp.AddTool(server, msi.ListDirectory, ts.ListDirectory)
	mcp.AddTool(server, msi.ReadFile, ts.ReadFile)
	mcp.AddTool(server, msi.RunShellCommand, ts.RunShellCommand)
	return nil
}

func (ts *ToolSet) Close() error {
	var err error
	if ts.sftp != nil {
		err = errors.Join(err, ts.sftp.Close())
	}
	if ts.sftpCmd != nil && ts.sftpCmd.Process != nil {
		err = errors.Join(err, ts.sftpCmd.Process.Kill())
	}
	return err
}

func (ts *ToolSet) TranslateHostPath(hostPath string) (string, error) {
	if hostPath == "" {
		return "", errors.New("path is empty")
	}
	if !filepath.IsAbs(hostPath) {
		return "", fmt.Errorf("expected an absolute path, got a relative path: %q", hostPath)
	}
	// TODO: make sure that hostPath is mounted
	return hostPath, nil
}
