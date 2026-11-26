---
name: lima-agent-sandbox
description: Use a Lima VM as a sandbox for an AI coding agent, so the agent cannot read, write or execute host files outside the directory it was given. Use when asked to run Claude Code, Codex, Gemini, Antigravity, OpenCode, Aider or GitHub Copilot CLI inside a VM, to sandbox an agent or an untrusted script, to run an agent against a synced copy of a project rather than the original, or to connect an agent on the host to a Lima VM over MCP.
compatibility: Requires Lima >= 2.3 on the host; the --sync workflow requires Lima >= 2.1
license: Apache-2.0
metadata:
  lima-min-version: "2.3"
---

# Lima as an agent sandbox

Two arrangements, with different threat models:

| Arrangement | The agent runs | Isolation boundary |
|-------------|----------------|--------------------|
| Agent inside the VM | in the guest | the VM. The agent sees only what is mounted |
| Agent outside, VM as a tool | on the host | none for the agent itself; the VM is where its commands land |

Pick the first when the point is to contain the agent. Pick the second when the point is to
give a host agent somewhere safe to execute.

Reference: <https://lima-vm.io/docs/config/ai/> and
<https://lima-vm.io/docs/examples/ai/>.

## Decide: how much filesystem to expose

| Requirement | Do this |
|-------------|---------|
| The agent edits one project, directly | `limactl start --tty=false --mount-only "$PWD:w"` |
| The agent reads a project but must not write it | same, without `:w` (but see the krunkit caveat below) |
| The agent to work on a copy, not the original | `--mount-none`, then `limactl shell --sync` |
| The agent must not see the host filesystem at all | `--mount-none` |

Mount the project directory and nothing above it. A home directory, or any parent of the
project, defeats the purpose.

`--mount-none` removes the mounts Lima creates, and that is the whole guarantee. It is not a
filesystem boundary on the `wsl2` driver: WSL's own drive automount keeps every host drive
visible under `/mnt`, whatever the Lima mount list says. Use another driver when the point is
to keep host files out of the guest.

The mount type matters as much as the path. `reverse-sshfs`, the portable default, runs an SFTP
server on the host, and a compromised guest process can reach host directories that were never
mounted through it. When the boundary itself is the goal, prefer `virtiofs` where the driver
offers it and say that it is a smaller surface, not a proven one.

A read-only mount is not a security boundary on every driver. Under `vmType: krunkit`,
virtiofs mounts cannot be enforced read-only even with `writable: false`, because the guest
can remount them writable. If the agent must not write, do not mount the directory at all.

## Do: agent inside the VM

```bash
limactl start --tty=false --name agent --mount-only "$PWD:w" template:default
limactl shell agent
```

Then install the agent CLI in the guest. Lima's docs carry current install lines for Aider,
Claude Code, Codex, Gemini, GitHub Copilot CLI and OpenCode:
<https://lima-vm.io/docs/examples/ai/>.

Credentials: most of these CLIs authenticate interactively on first run inside the guest. When
a key is needed instead, set it in the guest shell profile rather than passing it from the
host, so it does not leak into the host environment or the instance's stored configuration.

That interactive first run does not fit an unattended pipeline. In CI the agent CLI has to be
installed and authenticated from a secret the runner already holds, which means the secret does
reach the guest. Say so rather than implying the VM keeps it from the host, and prefer a
short-lived token.

## Do: run the agent on a copy with `--sync`

`limactl shell --sync` copies the host working directory into the guest, runs the command
there, and syncs the result back on exit. Lima 2.1+, rsync on both sides, and an instance
created without host mounts.

```bash
limactl start --tty=false --name sealed --mount-none template:default
cd ~/my-project
limactl shell --sync . sealed
```

**The accept/discard prompt only appears when stdout is a terminal.** Redirecting or piping
the output is enough to lose it, so `limactl shell --sync . sealed | tee build.log` applies the
changes without asking even with a human sitting there. Under `--tty=false` or `-y` it never
prompts either. In automation `--sync` is an automatic apply, not a review gate.

So when the output is not a terminal:

- run it on a git-clean working tree and review afterwards with `git diff`
- or take your own copy of the host directory first, with `cp -a` or `git stash`

`limactl snapshot` is not that copy: it snapshots the guest disk, not your host files.

### When nothing may land before a human approves

`--sync` cannot give you that: by the time it returns, the files are already written. Two ways
to get a real gate.

Point `--sync` at a throwaway checkout rather than the working tree the user cares about:

```bash
git clone --no-hardlinks . /deep/enough/path/agent-run
cd /deep/enough/path/agent-run
limactl shell --sync . sealed <agent command>
git -C /deep/enough/path/agent-run diff      # review, then cherry-pick or discard
```

Use a real clone, not `git worktree add`. A linked worktree's `.git` is a file that points at the
original repository's metadata, which does not exist inside the guest copy, so every `git`
command fails there. Cloning costs one object copy and keeps the review gate working.

Or keep the result in the guest and bring back only a patch:

```bash
limactl start --tty=false --name sealed --mount-none template:default
limactl shell sealed sh -c 'cd <repo in guest> && git add -A && git diff --cached --binary' > /tmp/agent.patch
# review /tmp/agent.patch, then on the host:
git apply /tmp/agent.patch
```

Stage before diffing. Plain `git diff` omits untracked files, so a file the agent created would be
missing from the patch with no warning; `--cached --binary` carries new and binary files too.

Answering **No** at the prompt does more than skip the sync: Lima then deletes the guest copy.
If the work might be worth keeping, choose "View the changed contents" or copy it out of the
guest before declining.

**Requires Lima >= 2.3.** On an older host `--sync-exclude` is an unknown flag and the command
fails before it runs. Check with `limactl --version` first.

Keep large or sensitive directories out of the copy with `--sync-exclude` (repeatable) or a
`.limasyncignore` file in the synced directory. Flags must come **before** the instance name:
`limactl shell` stops parsing options at the first positional argument, so anything after the
instance is part of the guest command.

```bash
limactl shell --sync . --sync-exclude .git --sync-exclude node_modules sealed make build
```

Preconditions that are enforced and fail the command outright:

- the host directory must be at least four path segments deep, so `/tmp/x` and shallow CI
  checkouts are rejected with "cd to a deeper directory"
- `--sync` is refused on `wsl2` instances, where the host is already visible through `/mnt`

## Do: agent outside, VM as a tool

Lima ships an MCP server that exposes shell and filesystem tools scoped to one running
instance:

```bash
limactl start --tty=false --mount-only "$PWD:w"
limactl mcp info
```

Do not run `limactl mcp serve` yourself: it is a blocking stdio server that the MCP client
starts. Wiring it into a specific client is per-client, so see
[references/mcp-clients.md](references/mcp-clients.md).

## Verify

```bash
limactl shell agent pwd
limactl shell agent ls /
limactl list agent --format json | jq '.config.mounts'
```

Confirm the mount list contains only what was intended before letting an agent loose.

## When it breaks

| Signature | Cause | Fix |
|-----------|-------|-----|
| The agent cannot see the project | the directory was not mounted, or `--mount-none` was used | check `.config.mounts` as above |
| The agent cannot write | the mount has no `:w` | recreate with `--mount-only "<dir>:w"` |
| `--sync` refuses to run | the instance has host mounts, or rsync is missing | recreate with `--mount-none`; install rsync on both sides |
| `--sync` fails with "cd to a deeper directory" | the host directory is fewer than four path segments deep | work from a deeper path; `/tmp/x` will not do |
| `--sync` is rejected outright | the instance is `wsl2`, where `/mnt` already exposes the host | use mounts instead |
| Host files changed although the prompt was declined | there was no prompt: no TTY | expected under `-y` or `--tty=false`; see above |
| The agent CLI is missing from the guest | nothing is preinstalled | install it in the guest, per the Lima AI examples page |
| MCP tools do not appear in the client | the client config points at the wrong command or instance | [references/mcp-clients.md](references/mcp-clients.md) |

## What this does and does not protect

A VM boundary is stronger than a container namespace, and the operator chooses the guest's
security posture. It is still not a defence against a determined attacker with a hypervisor
escape. Treat it as containment of mistakes and of ordinary untrusted code, not as a
guarantee.

Network access from the guest is unrestricted by default. An agent in a VM can still reach the
internet and anything on the host network.
