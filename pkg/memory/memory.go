// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	hostagentclient "github.com/lima-vm/lima/v2/pkg/hostagent/api/client"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func GetCurrent(ctx context.Context, inst *limatype.Instance) (int64, error) {
	var memory int64
	hostAgentPID, err := store.ReadPIDFile(filepath.Join(inst.Dir, filenames.HostAgentPID))
	if err != nil {
		return 0, err
	}
	if hostAgentPID != 0 {
		haSock := filepath.Join(inst.Dir, filenames.HostAgentSock)
		haClient, err := hostagentclient.NewHostAgentClient(haSock)
		if err != nil {
			return 0, err
		}
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		memory, err = haClient.GetCurrentMemory(ctx)
		if err != nil {
			return 0, err
		}
	}

	sshExe, err := sshutil.NewSSHExe()
	if err != nil {
		return 0, err
	}
	sshOpts, err := sshutil.CommonOpts(ctx, sshExe, false)
	if err != nil {
		return 0, err
	}
	sshArgs := append(sshutil.SSHArgsFromOpts(sshOpts),
		"-p", fmt.Sprintf("%d", inst.SSHLocalPort),
		fmt.Sprintf("%s@%s", *inst.Config.User.Name, inst.SSHAddress),
	)

	args := []string{"cat", "/proc/meminfo"}
	sshCmd := exec.CommandContext(ctx, sshExe.Exe, append(sshArgs, args...)...)
	out, err := sshCmd.Output()
	if err != nil {
		return 0, err
	}

	var available int64
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "MemAvailable: ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return 0, fmt.Errorf("unexpected line: %s", line)
		}
		num, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0, err
		}
		if fields[2] == "kB" {
			num *= 1024
		}
		available = num
	}

	return memory - available, nil
}

func SetTarget(ctx context.Context, inst *limatype.Instance, memory int64) error {
	hostAgentPID, err := store.ReadPIDFile(filepath.Join(inst.Dir, filenames.HostAgentPID))
	if err != nil {
		return err
	}
	if hostAgentPID != 0 {
		haSock := filepath.Join(inst.Dir, filenames.HostAgentSock)
		haClient, err := hostagentclient.NewHostAgentClient(haSock)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		err = haClient.SetTargetMemory(ctx, memory)
		if err != nil {
			return err
		}
	}

	return nil
}
