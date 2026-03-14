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

type ReadFileResult struct {
	Content string `json:"content" jsonschema:"The content of the file."`
}

type ReadFileParams struct {
	Path string `json:"path" jsonschema:"The absolute path to the file to read."`
	// TODO: Offset *int   `json:"offset,omitempty" jsonschema:"For text files, the 0-based line number to start reading from. Requires limit to be set."`
	// TODO: Limit  *int   `json:"limit,omitempty" jsonschema:"For text files, the maximum number of lines to read. If omitted, reads a default maximum (e.g., 2000 lines) or the entire file if feasible."`
}

var WriteFile = &mcp.Tool{
	Name:        "write_file",
	Description: `Writes content to a specified file. If the file exists, it will be overwritten. If the file doesn't exist, it (and any necessary parent directories) will be created.`,
}

type WriteFileResult struct {
	// Empty for now
}

type WriteFileParams struct {
	Path    string `json:"path" jsonschema:"The absolute path to the file to write to."`
	Content string `json:"content" jsonschema:"The content to write into the file."`
}

var Glob = &mcp.Tool{
	Name:        "glob",
	Description: `Finds files matching specific glob patterns (e.g., src/**/*.ts, *.md)`, // Not sorted by mod time, unlike Gemini
}

type GlobParams struct {
	Pattern string  `json:"pattern" jsonschema:"The glob pattern to match against (e.g., '*.py', 'src/**/*.js')."`
	Path    *string `json:"path,omitempty" jsonschema:"The absolute path to the directory to search within. If omitted, searches the tool's root directory."`
	// TODO: CaseSensitive bool    `json:"case_sensitive,omitempty" jsonschema:": Whether the search should be case-sensitive. Defaults to false."`
}

type GlobResult struct {
	Matches []string `json:"matches" jsonschema:"A list of absolute file paths that match the provided glob pattern."`
}

var SearchFileContent = &mcp.Tool{
	Name:        "search_file_content",
	Description: `Searches for a regular expression pattern within the content of files in a specified directory. Internally calls 'git grep -n --no-index'.`,
}

type SearchFileContentParams struct {
	Pattern string  `json:"pattern" jsonschema:"The regular expression (regex) to search for (e.g., 'function\\s+myFunction')."`
	Path    *string `json:"path,omitempty" jsonschema:"The absolute path to the directory to search within. Defaults to the current working directory."`
	Include *string `json:"include,omitempty" jsonschema:"A glob pattern to filter which files are searched (e.g., '*.js', 'src/**/*.{ts,tsx}'). If omitted, searches most files (respecting common ignores)."`
}

type SearchFileContentResult struct {
	GitGrepOutput string `json:"git_grep_output" jsonschema:"The raw output from the 'git grep -n --no-index' command, containing matching lines with filenames and line numbers."`
}

// TODO: implement Replace
