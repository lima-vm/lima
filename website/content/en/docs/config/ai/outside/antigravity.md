---
title: Antigravity
weight: 20
aliases:
  - /docs/config/ai/outside/gemini/
---

| ⚡ Requirement | Lima >= 2.0 |
|---------------|-------------|

This page describes how to use Lima as a sandbox for [Google Antigravity CLI](https://antigravity.google/docs/getting-started?tab=cli),
the successor to Gemini CLI.

## Prerequisite
In addition to Antigravity CLI and Lima, make sure that `limactl mcp` plugin is installed:

```console
$ limactl mcp -v
limactl-mcp version 2.0.0-alpha.1
```

The `limactl mcp` plugin is bundled in Lima since v2.0, however, it may not be installed
depending on the method of the [installation](../../../installation/).

## Configuration
1. Run the default Lima instance, with a mount of your project directory:
```bash
limactl start --mount-only "$(pwd):w" default
```

Drop the `:w` suffix if you do not want to allow writing to the mounted directory.

2. Register the Lima MCP server:
```bash
agy mcp add lima limactl mcp serve default
```

This stores the server in `~/.gemini/config/mcp_config.json`:
```json
{
  "mcpServers": {
    "lima": {
      "args": [
        "mcp",
        "serve",
        "default"
      ],
      "command": "limactl",
      "disabled": false
    }
  }
}
```

3. Modify `~/.gemini/antigravity-cli/settings.json` so as to deny Antigravity CLI's
[built-in permissions](https://antigravity.google/docs/permissions?tab=cli) for local command execution and file I/O,
while allowing web access and the MCP tools provided by Lima:
```json
{
  "permissions": {
    "allow": [
      "read_url(*)",
      "execute_url(*)",
      "mcp(lima/*)"
    ],
    "deny": [
      "command(*)",
      "read_file(*)",
      "write_file(*)"
    ]
  }
}
```

## Usage
Just run `agy` in your project directory.

Antigravity CLI automatically recognizes the MCP tools provided by Lima.
Type `/mcp` in the prompt panel to confirm that the `lima` MCP server is connected.
