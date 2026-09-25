// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package toolset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lima-vm/lima/v2/pkg/mcp/msi"
)

// readLimitBytes is the maximum size of a file that ReadFile and Replace read.
const readLimitBytes = 32 * 1024 * 1024

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
		res.Entries[i].Size = new(f.Size())
		res.Entries[i].Mode = new(f.Mode())
		res.Entries[i].ModTime = new(f.ModTime())
		res.Entries[i].IsDir = new(f.IsDir())
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
	lr := io.LimitReader(f, readLimitBytes)
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
	dir := filepath.Dir(guestPath)
	err = ts.sftp.MkdirAll(dir)
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

func (ts *ToolSet) Replace(_ context.Context,
	_ *mcp.CallToolRequest, args msi.ReplaceParams,
) (*mcp.CallToolResult, *msi.ReplaceResult, error) {
	if ts.inst == nil {
		return nil, nil, errors.New("instance not registered")
	}
	guestPath, err := ts.TranslateHostPath(args.Path)
	if err != nil {
		return nil, nil, err
	}
	expected := 1
	if args.ExpectedReplacements != nil {
		expected = *args.ExpectedReplacements
	}
	f, err := ts.sftp.Open(guestPath)
	if err != nil {
		return nil, nil, err
	}
	// Read one byte past the limit, so that a truncated read is never written back.
	b, err := io.ReadAll(io.LimitReader(f, readLimitBytes+1))
	f.Close()
	if err != nil {
		return nil, nil, err
	}
	if len(b) > readLimitBytes {
		return nil, nil, fmt.Errorf("file is larger than %d bytes", readLimitBytes)
	}
	content, err := replaceExact(string(b), args.OldString, args.NewString, expected)
	if err != nil {
		return nil, nil, err
	}
	// No O_CREATE: Replace never creates a file, and keeps the mode of the existing one.
	w, err := ts.sftp.OpenFile(guestPath, os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		return nil, nil, err
	}
	if _, err = w.Write([]byte(content)); err != nil {
		w.Close()
		return nil, nil, err
	}
	if err = w.Close(); err != nil {
		return nil, nil, err
	}
	res := &msi.ReplaceResult{
		Replacements: expected,
	}
	return &mcp.CallToolResult{
		// Gemini:
		// On success: `Successfully modified file: /path/to/file.txt (1 replacements).`
		StructuredContent: res,
	}, res, nil
}

// replaceExact replaces oldString with newString in content, and fails unless
// oldString occurs exactly expected times.
func replaceExact(content, oldString, newString string, expected int) (string, error) {
	if oldString == "" {
		return "", errors.New("old_string must not be empty")
	}
	if oldString == newString {
		return "", errors.New("old_string and new_string are identical")
	}
	if expected < 1 {
		return "", fmt.Errorf("expected_replacements must be at least 1, got %d", expected)
	}
	found := strings.Count(content, oldString)
	if found == 0 {
		return "", errors.New("old_string not found")
	}
	if found != expected {
		return "", fmt.Errorf("expected %d occurrence(s) of old_string, found %d", expected, found)
	}
	return strings.Replace(content, oldString, newString, expected), nil
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
	if matches == nil {
		matches = []string{}
	}
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
