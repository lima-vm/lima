---
title: Gemini
weight: 20

---

| ⚡ Requirement | Lima >= 2.0 |
|---------------|-------------|

This page describes how to use Lima as an sandbox for [Google Gemini CLI](https://github.com/google-gemini/gemini-cli).

## Prerequisite
In addition to Gemini and Lima, make sure that `limactl mcp` plugin is installed:

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

2. Create `.gemini/extensions/lima/gemini-extension.json` as follows:
```json
{
  "name": "lima",
  "version": "2.0.0",
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

3. Modify `.gemini/settings.json` so as to disable Gemini CLI's [built-in tools](https://github.com/google-gemini/gemini-cli/tree/main/docs/tools)
except ones that do not relate to local command execution and file I/O:
```json
{
  "coreTools": ["WebFetchTool", "WebSearchTool", "MemoryTool"]
}
```

## Usage
Just run `gemini` in your project directory.

Gemini automatically recognizes the MCP tools provided by Lima.
