// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Portion of AI prompt texts from:
// - https://github.com/google-gemini/gemini-cli/blob/v0.1.12/docs/tools/shell.md
//
// SPDX-FileCopyrightText: Copyright 2025 Google LLC

package msi

import "github.com/modelcontextprotocol/go-sdk/mcp"

var RunShellCommand = &mcp.Tool{
	Name:        "run_shell_command",
	Description: `Executes a given shell command.`,
}

type RunShellCommandParams struct {
	Command     []string `json:"command" jsonschema:"The exact shell command to execute. Defined as a string slice, unlike Gemini's run_shell_command that defines it as a single string."`
	Description string   `json:"description,omitempty" jsonschema:"A brief description of the command's purpose, which will be potentially shown to the user."`
	Directory   string   `json:"directory" jsonschema:"The absolute directory in which to execute the command. Unlike Gemini's run_shell_command, this must not be a relative path, and must not be empty."`
}

type RunShellCommandResult struct {
	Stdout   string `json:"stdout" jsonschema:"Output from the standard output stream."`
	Stderr   string `json:"stderr" jsonschema:"Output from the standard error stream."`
	Error    string `json:"error,omitempty" jsonschema:"Any error message reported by the subprocess."`
	ExitCode *int   `json:"exit_code,omitempty" jsonschema:"Exit code of the command."`
}
