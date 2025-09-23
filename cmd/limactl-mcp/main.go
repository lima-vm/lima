// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/lima-vm/lima/v2/pkg/limactlutil"
	"github.com/lima-vm/lima/v2/pkg/mcp/toolset"
	"github.com/lima-vm/lima/v2/pkg/version"
)

func main() {
	if err := newApp().Execute(); err != nil {
		logrus.Fatal(err)
	}
}

func newApp() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "limactl-mcp",
		Short:         "Model Context Protocol plugin for Lima (EXPERIMENTAL)",
		Version:       strings.TrimPrefix(version.Version, "v"),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(
		newMcpInfoCommand(),
		newMcpServeCommand(),
		newMcpGenDocCommand(),
		// TODO: `limactl-mcp configure gemini` ?
	)
	return cmd
}

func newServer() *mcp.Server {
	impl := &mcp.Implementation{
		Name:    "lima",
		Title:   "Lima VM, for sandboxing local command executions and file I/O operations",
		Version: version.Version,
	}
	serverOpts := &mcp.ServerOptions{
		Instructions: `This MCP server provides tools for sandboxing local command executions and file I/O operations,
by wrapping them in Lima VM (https://lima-vm.io).

Use these tools to avoid accidentally executing malicious codes directly on the host.
`,
	}
	if runtime.GOOS != "linux" {
		serverOpts.Instructions += fmt.Sprintf(`

NOTE: the guest OS of the VM is Linux, while the host OS is %s.
`, cases.Title(language.English).String(runtime.GOOS))
	}
	return mcp.NewServer(impl, serverOpts)
}

func newMcpInfoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show information about the MCP server",
		Args:  cobra.NoArgs,
		RunE:  mcpInfoAction,
	}
	return cmd
}

func mcpInfoAction(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	info, err := inspectInfo(ctx)
	if err != nil {
		return err
	}
	j, err := json.MarshalIndent(info, "", "    ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), string(j))
	return err
}

func inspectInfo(ctx context.Context) (*Info, error) {
	ts, err := toolset.New("")
	if err != nil {
		return nil, err
	}
	server := newServer()
	if err = ts.RegisterServer(server); err != nil {
		return nil, err
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		return nil, err
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "client"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		return nil, err
	}
	toolsResult, err := clientSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, err
	}
	if err = clientSession.Close(); err != nil {
		return nil, err
	}
	if err = serverSession.Wait(); err != nil {
		return nil, err
	}
	info := &Info{
		Tools: toolsResult.Tools,
	}
	return info, nil
}

type Info struct {
	Tools []*mcp.Tool `json:"tools"`
}

func newMcpServeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve INSTANCE",
		Short: "Serve MCP over stdio",
		Long: `Serve MCP over stdio.

Expected to be executed via an AI agent, not by a human`,
		Args: cobra.MaximumNArgs(1),
		RunE: mcpServeAction,
	}
	return cmd
}

func mcpServeAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	instName := "default"
	if len(args) > 0 {
		instName = args[0]
	}
	limactl, err := limactlutil.Path()
	if err != nil {
		return err
	}
	// FIXME: We can not use store.Inspect() here because it requires VM drivers to be compiled in.
	// https://github.com/lima-vm/lima/pull/3744#issuecomment-3289274347
	inst, err := limactlutil.Inspect(ctx, limactl, instName)
	if err != nil {
		return err
	}
	if len(inst.Errors) != 0 {
		return errors.Join(inst.Errors...)
	}
	ts, err := toolset.New(limactl)
	if err != nil {
		return err
	}
	server := newServer()
	if err = ts.RegisterServer(server); err != nil {
		return err
	}
	if err = ts.RegisterInstance(ctx, inst); err != nil {
		return err
	}
	transport := &mcp.StdioTransport{}
	return server.Run(ctx, transport)
}

func newMcpGenDocCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "generate-doc DIR",
		Short:  "Generate documentation pages",
		Args:   cobra.MinimumNArgs(1),
		RunE:   mcpGenDocAction,
		Hidden: true,
	}
	return cmd
}

func mcpGenDocAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	dir := args[0]
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	fName := filepath.Join(dir, "mcp.md")
	f, err := os.Create(fName)
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprint(f, `---
title: MCP tools
weight: 99
---
Lima implements the "MCP Sandbox Interface" (tentative name):
https://pkg.go.dev/github.com/lima-vm/lima/v2/pkg/mcp/msi

MCP Sandbox Interface defines MCP (Model Context Protocol) tools
that can be used for reading, writing, and executing local files
with an appropriate sandboxing technology, such as Lima.

The sandboxing technology can be more secure and/or efficient than
the default tools provided by an AI agent.

MCP Sandbox Interface was inspired by
[Google Gemini CLI's built-in tools](https://github.com/google-gemini/gemini-cli/tree/main/docs/tools).

`)
	info, err := inspectInfo(ctx)
	if err != nil {
		return err
	}
	for _, tool := range info.Tools {
		fmt.Fprintf(f, "## `%s`\n\n", tool.Name)
		if tool.Title != "" {
			fmt.Fprintf(f, "### Title\n\n%s\n\n", tool.Title)
		}
		if tool.Description != "" {
			fmt.Fprintf(f, "### Description\n\n%s\n\n", tool.Description)
		}
		if tool.InputSchema != nil {
			fmt.Fprint(f, "### Input Schema\n\n")
			schema, err := json.MarshalIndent(tool.InputSchema, "", "    ")
			if err != nil {
				return err
			}
			fmt.Fprintf(f, "```json\n%s\n```\n\n", string(schema))
		}
		if tool.OutputSchema != nil {
			fmt.Fprint(f, "### Output Schema\n\n")
			schema, err := json.MarshalIndent(tool.OutputSchema, "", "    ")
			if err != nil {
				return err
			}
			fmt.Fprintf(f, "```json\n%s\n```\n\n", string(schema))
		}
	}
	return f.Close()
}
