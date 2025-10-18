// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"context"
	"path/filepath"
	"time"

	hostagentclient "github.com/lima-vm/lima/v2/pkg/hostagent/api/client"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/store"
)

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
