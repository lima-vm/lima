// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package toolset

import (
	"context"
	"errors"
	"io"
	"os"
	"path"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lima-vm/lima/v2/pkg/mcp/msi"
	"github.com/lima-vm/lima/v2/pkg/ptr"
)

func (ts *ToolSet) ListDirectory(ctx context.Context,
	_ *mcp.CallToolRequest, args msi.ListDirectoryParams,
) (*mcp.CallToolResult, *msi.ListDirectoryResult, error) {
	if ts.inst == nil {
		return nil, nil, errors.New("instance not registered")
	}
	guestPath, err := ts.TranslateHostPath(args.Path)
	if err != nil {
		return nil, nil, err
	}
	guestEnts, err := ts.sftp.ReadDirContext(ctx, guestPath)
	if err != nil {
		return nil, nil, err
	}
	res := &msi.ListDirectoryResult{
		Entries: make([]msi.ListDirectoryResultEntry, len(guestEnts)),
	}
	for i, f := range guestEnts {
		res.Entries[i].Name = f.Name()
		res.Entries[i].Size = ptr.Of(f.Size())
		res.Entries[i].Mode = ptr.Of(f.Mode())
		res.Entries[i].ModTime = ptr.Of(f.ModTime())
		res.Entries[i].IsDir = ptr.Of(f.IsDir())
	}
	return &mcp.CallToolResult{
		StructuredContent: res,
	}, res, nil
}

func (ts *ToolSet) ReadFile(_ context.Context,
	_ *mcp.CallToolRequest, args msi.ReadFileParams,
) (*mcp.CallToolResult, *msi.ReadFileResult, error) {
	if ts.inst == nil {
		return nil, nil, errors.New("instance not registered")
	}
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
	res := &msi.ReadFileResult{
		Content: string(b),
	}
	return &mcp.CallToolResult{
		// Gemini:
		// For text files: The file content, potentially prefixed with a truncation message
		// (e.g., [File content truncated: showing lines 1-100 of 500 total lines...]\nActual file content...).
		StructuredContent: res,
	}, res, nil
}

func (ts *ToolSet) WriteFile(_ context.Context,
	_ *mcp.CallToolRequest, args msi.WriteFileParams,
) (*mcp.CallToolResult, *msi.WriteFileResult, error) {
	if ts.inst == nil {
		return nil, nil, errors.New("instance not registered")
	}
	guestPath, err := ts.TranslateHostPath(args.Path)
	if err != nil {
		return nil, nil, err
	}
	f, err := ts.sftp.Create(guestPath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	_, err = f.Write([]byte(args.Content))
	if err != nil {
		return nil, nil, err
	}
	res := &msi.WriteFileResult{}
	return &mcp.CallToolResult{
		// Gemini:
		// A success message, e.g., `Successfully overwrote file: /path/to/your/file.txt`
		// or `Successfully created and wrote to new file: /path/to/new/file.txt.`
		StructuredContent: res,
	}, res, nil
}

func (ts *ToolSet) Glob(_ context.Context,
	_ *mcp.CallToolRequest, args msi.GlobParams,
) (*mcp.CallToolResult, *msi.GlobResult, error) {
	if ts.inst == nil {
		return nil, nil, errors.New("instance not registered")
	}
	pathStr, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	if args.Path != nil && *args.Path != "" {
		pathStr = *args.Path
	}
	guestPath, err := ts.TranslateHostPath(pathStr)
	if err != nil {
		return nil, nil, err
	}
	pattern := path.Join(guestPath, args.Pattern)
	matches, err := ts.sftp.Glob(pattern)
	if err != nil {
		return nil, nil, err
	}
	res := &msi.GlobResult{
		Matches: matches,
	}
	return &mcp.CallToolResult{
		// Gemini:
		// A message like: Found 5 file(s) matching "*.ts" within src, sorted by modification time (newest first):\nsrc/file1.ts\nsrc/subdir/file2.ts...
		StructuredContent: res,
	}, res, nil
}

func (ts *ToolSet) SearchFileContent(ctx context.Context,
	req *mcp.CallToolRequest, args msi.SearchFileContentParams,
) (*mcp.CallToolResult, *msi.SearchFileContentResult, error) {
	if ts.inst == nil {
		return nil, nil, errors.New("instance not registered")
	}
	pathStr, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	if args.Path != nil && *args.Path != "" {
		pathStr = *args.Path
	}
	guestPath, err := ts.TranslateHostPath(pathStr)
	if err != nil {
		return nil, nil, err
	}
	if args.Include != nil && *args.Include != "" {
		guestPath = path.Join(guestPath, *args.Include)
	}
	cmdToolRes, cmdRes, err := ts.RunShellCommand(ctx, req, msi.RunShellCommandParams{
		Command:   []string{"git", "grep", "-n", "--no-index", args.Pattern, guestPath},
		Directory: pathStr, // Directory must be always set
	})
	if err != nil {
		return cmdToolRes, nil, err
	}
	res := &msi.SearchFileContentResult{
		GitGrepOutput: cmdRes.Stdout,
	}
	return &mcp.CallToolResult{
		// Gemini:
		// A message like: Found 10 matching lines for regex "function\\s+myFunction" in directory src:\nsrc/file1.js:10:function myFunction() {...}\nsrc/subdir/file2.ts:45:    function myFunction(param) {...}...
		StructuredContent: res,
	}, res, nil
}
