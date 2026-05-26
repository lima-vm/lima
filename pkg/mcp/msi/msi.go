// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package msi provides the "MCP Sandbox Interface" (tentative)
// that should be reusable for other projects too.
//
// MCP Sandbox Interface defines MCP (Model Context Protocol) tools
// that can be used for reading, writing, and executing local files
// with an appropriate sandboxing technology. The sandboxing technology
// can be more secure and/or efficient than the default tools provided
// by an AI agent.
//
// MCP Sandbox Interface was inspired by Gemini CLI's built-in tools.
// https://github.com/google-gemini/gemini-cli/tree/v0.1.12/docs/tools
//
// Notable differences from Gemini CLI's built-in tools:
//   - the output format is JSON, not a plain text
//   - the output of [SearchFileContent] always corresponds to `git grep -n --no-index`
//   - [RunShellCommandParams].Command is a string slice, not a string
//   - [RunShellCommandParams].Directory is an absolute path, not a relative path
//   - [RunShellCommandParams].Directory must not be empty
//
// Eventually, this package may be split to a separate repository.
package msi
