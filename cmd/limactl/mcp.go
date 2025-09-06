// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/mcp/toolset"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/version"
)

func newMcpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mcp",
		Short:   "Model Context Protocol",
		GroupID: advancedCommand,
	}
	cmd.AddCommand(
		newMcpServeCommand(),
		// TODO: `limactl mcp install-gemini` ?
	)
	return cmd
}

func newMcpServeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve INSTANCE",
		Short: "Serve MCP over stdio",
		Long: `Serve MCP over stdio.

Expected to be executed via an AI agent, not by a human`,
		Args: WrapArgsError(cobra.MaximumNArgs(1)),
		RunE: mcpServeAction,
	}
	return cmd
}

func mcpServeAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	instName := DefaultInstanceName
	if len(args) > 0 {
		instName = args[0]
	}
	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		return err
	}
	ts, err := toolset.New(ctx, inst)
	if err != nil {
		return err
	}
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
`, strings.ToTitle(runtime.GOOS))
	}
	server := mcp.NewServer(impl, serverOpts)
	if err = ts.RegisterServer(server); err != nil {
		return err
	}
	transport := &mcp.StdioTransport{}
	return server.Run(ctx, transport)
}
