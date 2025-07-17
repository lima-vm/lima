// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package toolset

import (
	"context"
	"encoding/json"
	"io"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lima-vm/lima/v2/pkg/mcp/msi"
	"github.com/lima-vm/lima/v2/pkg/ptr"
)

func (ts *ToolSet) ListDirectory(ctx context.Context,
	_ *mcp.CallToolRequest, args msi.ListDirectoryParams,
) (*mcp.CallToolResult, any, error) {
	guestPath, err := ts.TranslateHostPath(args.Path)
	if err != nil {
		return nil, nil, err
	}
	guestEnts, err := ts.sftp.ReadDirContext(ctx, guestPath)
	if err != nil {
		return nil, nil, err
	}
	res := msi.ListDirectoryResult{
		Entries: make([]msi.ListDirectoryResultEntry, len(guestEnts)),
	}
	for i, f := range guestEnts {
		res.Entries[i].Name = f.Name()
		res.Entries[i].Size = ptr.Of(f.Size())
		res.Entries[i].Mode = ptr.Of(f.Mode())
		res.Entries[i].ModTime = ptr.Of(f.ModTime())
		res.Entries[i].IsDir = ptr.Of(f.IsDir())
	}
	resJ, err := json.Marshal(res)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(resJ)}},
	}, nil, nil
}

func (ts *ToolSet) ReadFile(_ context.Context,
	_ *mcp.CallToolRequest, args msi.ReadFileParams,
) (*mcp.CallToolResult, any, error) {
	guestPath, err := ts.TranslateHostPath(args.Path)
	if err != nil {
		return nil, nil, err
	}
	f, err := ts.sftp.Open(guestPath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	const limitBytes = 32 * 1024 * 1024
	lr := io.LimitReader(f, limitBytes)
	b, err := io.ReadAll(lr)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}
