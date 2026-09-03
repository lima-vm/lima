---
title: Antigravity
weight: 20

---

| ⚡ Requirement | Lima >= 2.0 |
|---------------|-------------|

This page describes how to use Lima as a sandbox for [Google Antigravity CLI](https://github.com/google-antigravity/antigravity-cli).

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

2. Create `~/.gemini/antigravity-cli/mcp_config.json` as follows:
```json
{
  "mcpServers": {
    "lima": {
      "command": "limactl",
      "args": [
        "mcp",
        "serve",
        "default"
      ]
    }
  }
}
```

3. Modify `~/.gemini/antigravity-cli/settings.json` to configure the permissions granted to Antigravity CLI. For example:
```json
{
  "permissions": {
    "allow": [],
    "ask": [],
    "deny": []
  }
}
```

## Usage
Just run `agy` in your project directory.

Antigravity CLI automatically recognizes the configured MCP server provided by Lima.
