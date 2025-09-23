// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package toolset

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lima-vm/lima/v2/pkg/mcp/msi"
	"github.com/lima-vm/lima/v2/pkg/ptr"
)

func (ts *ToolSet) RunShellCommand(ctx context.Context,
	_ *mcp.CallToolRequest, args msi.RunShellCommandParams,
) (*mcp.CallToolResult, any, error) {
	if ts.inst == nil {
		return nil, nil, errors.New("instance not registered")
	}
	guestPath, err := ts.TranslateHostPath(args.Directory)
	if err != nil {
		return nil, nil, err
	}
	cmd := exec.CommandContext(ctx, ts.limactl,
		append([]string{"shell", "--workdir=" + guestPath, ts.inst.Name},
			args.Command...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmdErr := cmd.Run()
	res := msi.RunShellCommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if cmdErr == nil {
		res.ExitCode = ptr.Of(0)
	} else {
		res.Error = cmdErr.Error()
		if st := cmd.ProcessState; st != nil {
			res.ExitCode = ptr.Of(st.ExitCode())
		}
	}
	resJ, err := json.Marshal(res)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(resJ)}},
	}, nil, nil
}
