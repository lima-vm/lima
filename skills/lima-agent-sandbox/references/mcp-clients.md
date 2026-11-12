# Connecting an MCP client to a Lima instance

`limactl mcp serve <instance>` speaks the Model Context Protocol over stdio and exposes tools
that run shell commands and read and write files **inside one already-running instance**. It
is started by the client, not by hand.

Status: experimental, Lima 2.0+. Check it is installed:

```bash
limactl mcp -v
```

If that fails, the `limactl-mcp` plugin was not included by the installation method in use.
See <https://lima-vm.io/docs/installation/>.

## Before wiring anything

The instance must exist, be running, and have the project mounted. The MCP tools translate
host paths to guest paths, so a directory the guest cannot see is a directory the tools cannot
reach.

```bash
limactl start --tty=false --name <instance> --mount-only "$PWD:w"
```

## The server entry

Every client needs the same three facts. Only the file and its syntax differ.

| Field | Value |
|-------|-------|
| transport | stdio |
| command | `limactl`, or its absolute path if the client does not inherit your shell `PATH` |
| arguments | `mcp`, `serve`, `<instance>` |

In the JSON shape most clients use:

```json
{
  "mcpServers": {
    "lima": {
      "command": "limactl",
      "args": ["mcp", "serve", "<instance>"]
    }
  }
}
```

The tools it exposes are `run_shell_command`, `list_directory`, `read_file`, `write_file`,
`glob`, `search_file_content` and `replace`. Knowing the names matters because the next step
is disabling the client's own equivalents.

## Per-client notes

**Antigravity** has a worked example in the Lima documentation, including the extension file
and the settings change that disables the client's own local execution tools:
<https://lima-vm.io/docs/config/ai/outside/antigravity/>.

**Other clients** take the same server entry in their own MCP configuration. Consult the
client's own documentation for the file path and syntax; they change faster than this page
can track.

## Turn off the client's local tools

This is the step that makes the arrangement worth anything. An MCP client that still has its
own shell and file-editing tools will keep using them, and the VM becomes decoration. Disable
the client's built-in local execution and file tools so the Lima tools are the only way it can
act.

The shape is the same wherever the client supports it. Deny the local tools, allow the Lima
ones. Antigravity spells it as:

```json
{
  "permissions": {
    "allow": ["read_url(*)", "execute_url(*)", "mcp(lima/*)"],
    "deny": ["command(*)", "read_file(*)", "write_file(*)"]
  }
}
```

The key names differ per client; the pattern of denying local execution and file I/O while
allowing `mcp(lima/*)` transfers. If a client cannot disable its own tools, this arrangement
does not contain anything, and the agent-inside-the-VM arrangement is the one to use.

## Verify

Ask the connected agent to run a command that reveals which machine it is on:

```
uname -a
```

The answer must describe the guest, not the host. If it names the host, the client is still
using its own tools.

## When it breaks

| Signature | Cause | Fix |
|-----------|-------|-----|
| `limactl mcp -v` fails | the plugin is not installed | reinstall Lima by a method that includes plugins |
| The client shows no Lima tools | wrong config path, or the client was not restarted | restart the client; check its MCP log |
| Tools appear but every call fails | the instance is not running | `limactl list <inst> --format json`, check `.status` |
| File tools cannot see the project | the directory is not mounted | recreate with `--mount-only "<dir>:w"` |
| The agent still edits host files | the client's own tools are still enabled | disable them, then re-run the verification above |
