// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Portion of AI prompt texts from:
// - https://github.com/google-gemini/gemini-cli/blob/v0.1.12/docs/tools/file-system.md
//
// SPDX-FileCopyrightText: Copyright 2025 Google LLC

package msi

import (
	"io/fs"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var ListDirectory = &mcp.Tool{
	Name:        "list_directory",
	Description: `Lists the names of files and subdirectories directly within a specified directory path.`,
}

type ListDirectoryParams struct {
	Path string `json:"path" jsonschema:"The absolute path to the directory to list."`
}

// ListDirectoryResultEntry is similar to [io/fs.FileInfo].
type ListDirectoryResultEntry struct {
	Name    string       `json:"name" jsonschema:"base name of the file"`
	Size    *int64       `json:"size,omitempty" jsonschema:"length in bytes for regular files; system-dependent for others"`
	Mode    *fs.FileMode `json:"mode,omitempty" jsonschema:"file mode bits"`
	ModTime *time.Time   `json:"time,omitempty" jsonschema:"modification time"`
	IsDir   *bool        `json:"is_dir,omitempty" jsonschema:"true for a directory"`
}

type ListDirectoryResult struct {
	Entries []ListDirectoryResultEntry `json:"entries" jsonschema:"The directory content entries."`
}

var ReadFile = &mcp.Tool{
	Name:        "read_file",
	Description: `Reads and returns the content of a specified file.`,
}

type ReadFileParams struct {
	Path string `json:"path" jsonschema:"The absolute path to the file to read."`
	// Offset *int   `json:"offset,omitempty" jsonschema:"For text files, the 0-based line number to start reading from. Requires limit to be set."`
	// Limit  *int   `json:"limit,omitempty" jsonschema:"For text files, the maximum number of lines to read. If omitted, reads a default maximum (e.g., 2000 lines) or the entire file if feasible."`
}
