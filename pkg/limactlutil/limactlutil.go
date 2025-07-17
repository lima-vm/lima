// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limactlutil

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

// Path returns the path to the `limactl` executable.
func Path() (string, error) {
	limactl := cmp.Or(os.Getenv("LIMACTL"), "limactl")
	return exec.LookPath(limactl)
}

// Inspect runs `limactl list --json INST` and parses the output.
func Inspect(ctx context.Context, limactl, instName string) (*limatype.Instance, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, limactl, "list", "--json", instName)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run %v: stdout=%q, stderr=%q: %w", cmd.Args, stdout.String(), stderr.String(), err)
	}
	var inst limatype.Instance
	if err := json.Unmarshal(stdout.Bytes(), &inst); err != nil {
		return nil, err
	}
	return &inst, nil
}
