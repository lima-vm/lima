// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package toolset

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/instance/hostname"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/mcp/msi"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
)

func New(limactl string) (*ToolSet, error) {
	ts := &ToolSet{
		limactl: limactl,
	}
	return ts, nil
}

type ToolSet struct {
	limactl string // needed for `limactl shell --workdir`

	// Set on RegisterInstance()
	inst    *limatype.Instance
	sftp    *sftp.Client
	sftpCmd *exec.Cmd
}

func newSFTPClient(ctx context.Context, inst *limatype.Instance) (*sftp.Client, *exec.Cmd, error) {
	sshExe, err := sshutil.NewSSHExe()
	if err != nil {
		return nil, nil, err
	}
	args := slices.Concat(sshExe.Args, []string{"-F", filepath.Join(inst.Dir, filenames.SSHConfig), hostname.FromInstName(inst.Name), "-s", "sftp"})
	cmd := exec.CommandContext(ctx, sshExe.Exe, args...)
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

func (ts *ToolSet) RegisterInstance(ctx context.Context, inst *limatype.Instance) error {
	if inst.Status != limatype.StatusRunning {
		return fmt.Errorf("expected status of instance %q to be %q, got %q",
			inst.Name, limatype.StatusRunning, inst.Status)
	}
	if len(inst.Config.Mounts) == 0 {
		logrus.Warnf("instance %q has no mount", inst.Name)
	}
	ts.inst = inst
	var err error
	ts.sftp, ts.sftpCmd, err = newSFTPClient(ctx, inst)
	return err
}

func (ts *ToolSet) RegisterServer(server *mcp.Server) error {
	mcp.AddTool(server, msi.ListDirectory, ts.ListDirectory)
	mcp.AddTool(server, msi.ReadFile, ts.ReadFile)
	mcp.AddTool(server, msi.WriteFile, ts.WriteFile)
	mcp.AddTool(server, msi.Glob, ts.Glob)
	mcp.AddTool(server, msi.SearchFileContent, ts.SearchFileContent)
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
