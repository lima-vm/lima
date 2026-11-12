---
name: lima
description: Create and operate Lima Linux virtual machines from the host with limactl. Use when a task needs a throwaway or per-project VM, a container runtime (containerd, Podman, Docker) isolated from the host daemon, a local Kubernetes cluster, host-to-guest port forwarding or directory mounts, or when a limactl command hangs, times out or fails. Also use when writing, customising or sharing Lima templates.
compatibility: Requires Lima >= 2.0 on the host (macOS, Linux or Windows)
license: Apache-2.0
metadata:
  lima-min-version: "2.0"
---

# Lima

Lima runs Linux (and, experimentally, macOS, FreeBSD and Windows) virtual machines on a macOS,
Linux or Windows host. It is most often used to get a container runtime, a Kubernetes cluster,
or a disposable build environment that cannot reach host state.

Full documentation: <https://lima-vm.io/docs/>

## Mental model

Three processes, and knowing which one failed is most of debugging:

| Process | Runs on | Does |
|---------|---------|------|
| `limactl` | host | the CLI you invoke; starts and stops everything else |
| hostagent | host | owns the VM process, port forwarding and mounts; logs to `ha.stdout.log` and `ha.stderr.log` |
| guestagent | guest | reports listening ports back to the hostagent over a socket |

State lives in `$LIMA_HOME` (default `~/.lima`), one directory per instance:

| Path | Contents |
|------|----------|
| `$LIMA_HOME/<instance>/lima.yaml` | the resolved configuration |
| `$LIMA_HOME/<instance>/ha.stderr.log` | hostagent errors: port forwarding, mounts, guestagent |
| `$LIMA_HOME/<instance>/serial*.log` | guest console, for boot failures |
| `$LIMA_HOME/<instance>/sock/` | forwarded guest sockets, e.g. `docker.sock` |
| `$LIMA_HOME/<instance>/copied-from-guest/` | files pulled back by `copyToHost`, e.g. a kubeconfig |

Resolve the directory rather than assuming it: `limactl list <instance> --format '{{.Dir}}'`.

Because the hostagent owns port forwarding from outside the guest, **rebooting from inside the
guest leaves it behind**: the workload comes back healthy and every forwarded port stays dead.
Restart from the host with `limactl restart <instance>`, which is a full power cycle, so an
upgraded kernel is picked up and the forwards are rebuilt.

## Running limactl without a terminal

Lima prompts and opens editors when stdout is a TTY. Automation must turn that off.

- Pass `-y` (alias of `--tty=false`) to any command that may prompt, including `start`,
  `delete`, `edit` and `snapshot apply`. The exception is `limactl shell --sync`, where the
  prompt *is* the review step. It disappears whenever stdout is not a terminal, so a pipe or a
  redirect suppresses it with no flag at all, and the guest's changes are applied to the host
  unconditionally. Read the `lima-agent-sandbox` skill before using it.
- Pass `--timeout` to `limactl start`; the default wait can outlive a job.
- Read machine-readable state, never the table: `limactl list --format json`, narrowed with
  `--filter '.status == "Running"'`.
- `limactl --version` before following any version-gated step.

```bash
limactl start -y --name dev --timeout 10m template:default
limactl list dev --format json | jq -r '.status'
limactl shell dev uname -a
```

## Host safety

Lima exists to keep work away from host state. Preserve that.

1. Do not touch host state outside `$LIMA_HOME` and the directories explicitly mounted.
2. `delete`, `factory-reset`, `prune`, `snapshot apply` and `disk delete` destroy data. Say
   what will be lost and get confirmation before running them.
3. Do not mount a home directory, or any parent of the project, writable into a VM running
   untrusted code. Mount the project directory and nothing above it:
   `--mount-only "$PWD:w"`.
4. Prefer a named per-project instance over mutating the shared `default` instance.
   `limactl protect <inst>` refuses an accidental `delete` until `limactl unprotect`.
5. Never weaken an isolation setting to make a command succeed.

`limactl template`, `limactl snapshot`, `limactl tunnel` and `limactl mcp` are marked
experimental (<https://lima-vm.io/docs/releases/experimental/>), so their flags and output may
change between releases. Check `limactl --version` before depending on them in automation.

## Where to look next

| The task is about | Read |
|-------------------|------|
| running a build or a test suite in a VM | start with [references/instances.md](references/instances.md), then the runtime or language you need |
| creating, starting, cloning, resizing or snapshotting an instance; picking a distro | [references/instances.md](references/instances.md) |
| containers: containerd, Podman, Docker, Apptainer, BuildKit, and exposing a socket to the host | [references/containers.md](references/containers.md), and [references/templates.md](references/templates.md) when a specific distro is also required |
| a local Kubernetes cluster and wiring its kubeconfig | [references/kubernetes.md](references/kubernetes.md) |
| reaching a guest port from the host, or a host service from the guest | [references/networking.md](references/networking.md) |
| sharing directories, copying files, or slow filesystem I/O | [references/mounts.md](references/mounts.md) |
| writing or sharing a template, or installing software into a guest with `provision:` | [references/templates.md](references/templates.md) |
| it hangs, times out, or something stopped working | [references/troubleshooting.md](references/troubleshooting.md) |

Running an AI coding agent inside a VM is a separate skill: `lima-agent-sandbox`.
