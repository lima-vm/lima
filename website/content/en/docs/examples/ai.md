---
title: AI agents
weight: 5
---

Lima is useful for running AI agents inside a VM, so as to prevent agents
from directly reading, writing, or executing the host files.

For running AI agents, it is highly recommended to only mount your project directory (current directory)
into the VM:

{{< tabpane text=true >}}
{{% tab header="Lima v2.0+" %}}
```bash
limactl start --mount-only .:w
```

Drop `:w` for read-only mode.
{{% /tab %}}
{{% tab header="Lima v1.x" %}}
```bash
limactl start --set ".mounts=[{\"location\":\"$(pwd)\", \"writable\":true}]"
```

Set `writable` to `false` for read-only mode.
{{% /tab %}}
{{< /tabpane >}}

<!--
AI agents are sorted alphabetically.

Node.js is installed via snap, not apt, as AI agents often require a very recent release of Node.js.
-->
{{< tabpane text=true >}}
{{% tab header="Aider" %}}
```
lima sudo apt install -y pipx
lima pipx install aider-install
lima sh -c 'echo "export PATH=$PATH:$HOME/.local/bin" >>~/.bash_profile'
lima aider-install
lima aider
```

Follow the guide shown in the first session for authentication.

Alternatively, you can set environmental variables via:
<!-- TODO: change ${USER}.linux to ${USER}.guest after releasing v2.1 GA -->
```
lima vi "/home/${USER}.linux/.bash_profile"
```

See also <https://github.com/Aider-AI/aider>.
{{% /tab %}}
{{% tab header="Claude Code" %}}
```
lima sudo snap install node --classic
lima sudo npm install -g @anthropic-ai/claude-code
lima claude
```

Follow the guide shown in the first session for authentication.

Alternatively, you can set `export ANTHROPIC_API_KEY...` via:
```
lima vi "/home/${USER}.linux/.bash_profile"
```

See also <https://github.com/anthropics/claude-code>.
{{% /tab %}}
{{% tab header="Codex" %}}
```
lima sudo snap install node --classic
lima sudo npm install -g @openai/codex
lima codex
```

Follow the guide shown in the first session for authentication.

Alternatively, you can set `export OPENAI_API_KEY...` via:
```
lima vi "/home/${USER}.linux/.bash_profile"
```

See also <https://github.com/openai/codex>.
{{% /tab %}}
{{% tab header="Gemini" %}}
```
lima sudo snap install node --classic
lima sudo npm install -g @google/gemini-cli
lima gemini
```

Follow the guide shown in the first session for authentication.

Alternatively, you can set `export GEMINI_API_KEY...` via:
```
lima vi "/home/${USER}.linux/.bash_profile"
```

See also <https://github.com/google-gemini/gemini-cli>.
{{% /tab %}}
{{% tab header="GitHub Copilot" %}}
```
lima sudo snap install node --classic
lima sudo npm install -g @github/copilot
lima copilot
```

Type `/login` in the first session for authentication.

Alternatively, you can set `export GH_TOKEN=...` via:
```
lima vi "/home/${USER}.linux/.bash_profile"
```

See also <https://github.com/github/copilot-cli>.
{{% /tab %}}
{{% tab header="OpenCode" %}}
```
lima sudo snap install node --classic
lima sudo npm install -g opencode-ai
lima opencode
```

Type `/connect` in the first session for authentication.
Unlike other agents, this step is not necessary for OpenCode.

See also <https://github.com/anomalyco/opencode>.
{{% /tab %}}

{{< /tabpane >}}


# Protecting `.git` from the agent

 | ⚡ Requirement | Lima >= 2.4, Linux or macOS host |
 |----------------|----------------------------------|

A writable project mount lets the agent write `.git/hooks/*` or `.git/config` (e.g., `core.fsmonitor`),
which run on the host the next time you use `git` in the project.
`sshfs.readonlyNames` keeps the working tree writable, with live updates on both sides,
while `.git` stays read-only for the guest:

```yaml
# agent.yaml
# Unlike template:default, this base does not mount the home directory.
base: template:_images/ubuntu
mountType: "reverse-sshfs"
mountInotify: true
mounts:
- location: "~/my-project"
  writable: true
  sshfs:
    sftpDriver: "builtin"
    readonlyNames: [".git"]
```

```bash
limactl start --name=agent ./agent.yaml
cd ~/my-project
limactl shell agent claude
```

The project must be the only host directory mounted in the guest, and every mount must use `sftpDriver: "builtin"`.

This is enforced by the SFTP server running on the host, not by the guest, so it also holds when the agent gains root
in the guest.
`.git` is protected at any depth, including nested repositories and names differing only by case.
See [Filesystem mounts » Read-only names](../config/mount.md#read-only-names) for details.

The agent can still read `.git` and run `git status`, `git diff` and `git log`, but not `git commit`.
Review and commit its changes from the host.

Other files in the working tree remain writable by the agent, so be careful with files that tools run on the host:
- `core.hooksPath` pointing into the working tree (e.g., `.husky`)
- `.envrc` (direnv), `.vscode/tasks.json`, `Makefile`, `package.json` scripts, etc.

Setting `git config --global safe.bareRepository explicit` on the host also prevents `git` from using a bare repository
that the agent could create outside of `.git`.

# Syncing Working Directory

 | ⚡ Requirement | Lima >= 2.1 |
 |----------------|-------------|

The `--sync` flag for [`limactl shell`](../reference/limactl_shell) enables bidirectional synchronization of your host working directory with the guest VM. This is particularly useful when running AI agents (like Claude, Copilot, or Gemini) inside VMs to prevent them from accidentally modifying or breaking files on your host system.

### Comparison with `mount`

| Feature | Mounts (`--mount`/`--mount-only`) | Sync (`--sync`) |
|---|---|---|
| Purpose | Make host directories visible inside guest (bidirectional if write mode is enabled) | Temporary bidirectional sync of a working directory (guest changes merged back on accept) |
| Live updates | Yes | No |
| Safety | Lower (AI agents can access host files directly) | Higher (changes are reviewed before being applied to host) |
| Requires rsync | No | Yes |

### Usecase - Running AI Code Assistants Safely

1. Create an isolated instance for AI agents which must be started without host mounts for `--sync` to work:

```bash
limactl start --mount-none template:default
```

2. Navigate to your project

```bash
cd ~/my-project
```

3. Run an AI agent that modifies code:

```bash
limactl shell --sync . default claude "Add error handling to all functions"
```

Or simply shell into the instance and make changes:
```bash
limactl shell --sync . default
```

4. After running commands, you'll see an interactive prompt:

```
⚠️ Accept the changes?
→ Yes
  No
  View the changed contents
```

- **Yes**: Syncs changes back to your host and cleans up guest directory
- **No**: Discards changes and cleans up guest directory
- **View the changed contents**: Shows a diff of changes made by the agent

### Requirements

- **rsync** must be installed on both host and guest
- The host working directory must be at least 4 levels deep (e.g., `/Users/username/projects/myproject`)
- The instance must not have any host mounts configured (use `--mount-none` when creating)
- The instance must not use the `wsl2` VM type

## See also

- [Config » AI](../config/ai/)
- [Config » GPU](../config/gpu.md)
