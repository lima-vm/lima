# Instances

Creating, starting and disposing of VMs. Reference:
<https://lima-vm.io/docs/> and `limactl <command> --help`.

## Decide

| You need | Do this |
|----------|---------|
| A general-purpose Linux VM | `limactl start -y --name <inst>` (Ubuntu with containerd). Bare `limactl start -y` creates the shared `default` instance; keep that for throwaway experiments only |
| A specific distribution | `limactl start -y --name <inst> template:fedora` |
| An isolated VM for one project | always pass `--name <inst>`; do not reuse `default` |
| A VM with no mounts, no port forwarding, no containerd | add `--plain` |
| To know what templates exist | `limactl start --list-templates` |
| To know which drivers this host supports | `limactl start --list-drivers` |

<https://lima-vm.io/docs/templates/> lists every template with its stability tier: a star for
Tier 1 (regularly tested in CI), a white star for Tier 2. Everything else is tested only
occasionally. Prefer a Tier 1 template when a task must not flake.

## Do

```bash
# create and start in one step; the name is also the instance name
limactl start -y --name dev --timeout 10m template:fedora

# size it
limactl start -y --name dev --cpus 4 --memory 8 --disk 60 template:default   # GiB

# mount only this project, writable, and nothing else
limactl start -y --name dev --mount-only "$PWD:w" template:default

# template parameters, where the template declares them
limactl start -y --name dev --param containerdSnapshotter=false template:docker

# anything the flags do not cover, as a yq expression over the template
limactl start -y --name dev --set '.nestedVirtualization=true' template:default
```

Lifecycle:

```bash
limactl list --format json              # one JSON object per line, not an array
limactl list --filter '.status == "Running"' --quiet
limactl shell dev                       # interactive
limactl shell dev sh -c 'id && pwd'    # one command
limactl stop dev
limactl start dev                       # subsequent starts take the name only
limactl restart dev
limactl delete -y dev                   # destroys the disk
```

`limactl shell` starts in the host working directory if it is mounted. Use `--workdir` to
choose another, and `--start` to boot the instance if it is stopped.

## Copies and rollbacks

| Goal | Command | Note |
|------|---------|------|
| A second instance like this one | `limactl clone --tty=false dev dev2` | the instance must be stopped; `clone` warns that `-y` is deprecated |
| A restore point before a risky change | `limactl snapshot create dev --tag before` | **qemu and krunkit only** |
| Roll back | `limactl snapshot apply dev --tag before` | destructive, confirm first |
| List restore points | `limactl snapshot list dev` | |

**Snapshots are not available on every driver.** `qemu` and `krunkit` support them; `vz`,
`wsl2` and `hcs` return "unimplemented". `vz` is the default on macOS, so on a stock Mac
instance `snapshot create` fails. Check first, and fall back to `limactl clone` of a stopped
instance when you need a restore point:

```bash
limactl list dev --format '{{.VMType}}'
```

`limactl clone` copies an instance. `limactl copy` copies files. They are unrelated.

## Verify

```bash
limactl list dev --format json | jq -r '.status, .vmType, .arch'
limactl shell dev uname -sr
```

`.status` is `Running` when the instance is usable.

## When it breaks

| Signature | Cause | Fix |
|-----------|-------|-----|
| The command waits forever with no output | the instance is booting, or the hostagent is stuck | pass `--timeout`, then read `ha.stderr.log` and `serial*.log` (see [troubleshooting.md](troubleshooting.md)) |
| `instance "x" not found` | wrong name, or it was never created | `limactl list --format json` |
| An editor opens, or a prompt appears | stdout is a TTY | pass `-y` |
| Changes to a template are ignored on restart | `limactl start <name>` reuses the stored config | `limactl edit <name>`, or delete and recreate |
| `limactl delete` refuses | the instance is protected | `limactl unprotect <name>` first |
