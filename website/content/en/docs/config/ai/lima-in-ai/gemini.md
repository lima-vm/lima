---
title: Gemini
weight: 20

---

| ⚡ Requirement    | Lima >= 2.0 |
|-------------------|-------------|

This page describes how to use Lima as an sandbox for [Google Gemini CLI](https://github.com/google-gemini/gemini-cli).

## Configuration
1. Run the default Lima instance:
```bash
limactl start default
```

1. Create `.gemini/extensions/lima/gemini-extension.json` as follows:
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

1. Modify `.gemini/settings.json` so as to disable Gemini CLI's [built-in tools](https://github.com/google-gemini/gemini-cli/tree/main/docs/tools)
except ones that do not relate to local command execution and file I/O:
```json
{
  "coreTools": ["WebFetchTool", "WebSearchTool", "MemoryTool"]
}
```

## Usage
Just run `gemini`.

The project directory must be mounted inside the VM. i.e., typically it must be under the home directory.
