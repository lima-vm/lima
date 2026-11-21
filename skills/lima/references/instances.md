# Instances

Creating, starting and disposing of VMs. Reference:
<https://lima-vm.io/docs/> and `limactl <command> --help`.

## Decide

| You need | Do this |
|----------|---------|
| A general-purpose Linux VM | `limactl start --tty=false --name <inst>` (Ubuntu with containerd). Bare `limactl start --tty=false` creates the shared `default` instance; keep that for throwaway experiments only |
| A specific distribution | `limactl start --tty=false --name <inst> template:fedora` |
| An isolated VM for one project | always pass `--name <inst>`; do not reuse `default` |
| A VM with no mounts, no port forwarding, no containerd | add `--plain` |
| A guest of a different architecture | `--arch aarch64` with `--vm-type qemu`; see below |
| Nested virtualisation | `--nested-virt` |
| To know what templates exist | `limactl start --list-templates` |
| To know which drivers this host supports | `limactl start --list-drivers` |
| To know which flags exist at all | `limactl start --help`. It matches the installed binary, which these tables may not |

<https://lima-vm.io/docs/templates/> lists every template with its stability tier: a star for
Tier 1 (regularly tested in CI), a white star for Tier 2. Everything else is tested only
occasionally. Prefer a Tier 1 template when a task must not flake.

## Do

```bash
# create and start in one step; the name is also the instance name
limactl start --tty=false --name dev --timeout 10m template:fedora

# size it
limactl start --tty=false --name dev --cpus 4 --memory 8 --disk 60 template:default   # GiB

# mount only this project, writable, and nothing else
limactl start --tty=false --name dev --mount-only "$PWD:w" template:default

# template parameters, where the template declares them
limactl start --tty=false --name dev --param containerdSnapshotter=false template:docker

# anything the flags do not cover, as a yq expression over the template
limactl start --tty=false --name dev --set '.ssh.loadDotSSHPubKeys=false' template:default
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
limactl delete dev                      # requires a stopped instance, or --force; destroys the disk, no prompt
```

`limactl shell` starts in the host working directory if it is mounted. Use `--workdir` to
choose another, and `--start` to boot the instance if it is stopped.

## Copies and rollbacks

| Goal | Command | Note |
|------|---------|------|
| A second instance like this one | `limactl clone --tty=false dev dev2` | the instance must be stopped; `clone` warns that `-y` is deprecated |
| A restore point before a risky change | `limactl snapshot create dev --tag before` | **qemu and krunkit only** |
| Roll back | `limactl snapshot apply dev --tag before` | destructive and **does not prompt**; confirm with the user first |
| List restore points | `limactl snapshot list dev` | |

**Snapshots are not available on every driver, and the two that have them differ.** `qemu`
snapshots a running or stopped instance; `krunkit` requires the instance to be stopped first.
`vz`, `wsl2` and `hcs` return "unimplemented", and `vz` is the default on macOS, so on a stock
Mac instance `snapshot create` fails outright. Check before relying on it:

```bash
limactl list dev --format '{{.VMType}}'
```

Where snapshots are unavailable, clone a stopped instance instead, and swap the clone in with
`limactl rename` if you need to roll back under the original name:

```bash
limactl stop dev
limactl clone --tty=false dev dev-before          # the restore point
# ... upgrade dev, and if it breaks:
limactl stop dev
limactl rename dev dev-broken
limactl rename dev-before dev
limactl start dev
```

`limactl clone` copies an instance. `limactl copy` copies files. They are unrelated.

## Upgrading the guest OS

Upgrade with the guest's own package manager, and do not reboot from inside. The hostagent owns
port forwarding from outside the guest, so an in-guest reboot leaves every forwarded port dead
even though the workload comes back. Read the distro rather than guessing the package manager,
then restart from the host:

```bash
limactl shell <inst> cat /etc/os-release
limactl shell <inst> sudo <that distro's upgrade command>
limactl restart <inst>          # full power cycle: new kernel, forwards rebuilt
```

Roll back with a snapshot or a clone if the upgrade breaks something: see above.

## A guest of a different architecture

Lima runs a foreign-architecture guest under QEMU emulation:

```bash
limactl start --tty=false --name arm --vm-type qemu --arch aarch64 --plain template:default
```

**The architecture names are the ones `uname -m` prints on Linux.** The Go and OCI spellings are
rejected rather than normalised, so translate first:

| You would normally write | Lima requires |
|--------------------------|---------------|
| `amd64`, `x64`, `x86-64` | `x86_64` |
| `arm64` | `aarch64` |
| `arm`, `armhf`, `armv7` | `armv7l` |
| `ppc64le`, `riscv64`, `s390x` | unchanged |

`x86_64` and `aarch64` are the supported pair; `armv7l`, `ppc64le`, `riscv64` and `s390x` are
experimental, and containerd is not installed for them automatically. The confusing part is
that Lima *does* use `amd64` and `arm64` internally when it reports the host architecture, so
seeing those names elsewhere is not permission to pass them in.

| Point | Detail |
|-------|--------|
| Host package | port forwarding needs `lima-additional-guestagents` installed on the host, or the start fails with a hint naming that package |
| `--plain` | disables mounts, port forwarding and containerd. Emulation is slow enough that a stock start can time out, so start here and add things back |
| Speed | extremely slow. Raise `--timeout` |
| Check | `limactl start --help` lists the accepted values for the installed version |

For cross-architecture *container* builds, emulating a whole VM is usually the wrong tool.
Run a native VM and use binfmt inside it, as described at
<https://lima-vm.io/docs/config/multi-arch/>.

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
| An editor opens, or a prompt appears | stdout is a TTY | pass `--tty=false` |
| Changes to a template are ignored on restart | `limactl start <name>` reuses the stored config | `limactl edit <name>`, or delete and recreate |
| `limactl delete` refuses | the instance is running, or it is protected | stop it first (`--force` skips the check), or `limactl unprotect <name>` |
