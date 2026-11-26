# Mounts and files

Sharing host directories with a guest, and moving files either way. Reference:
<https://lima-vm.io/docs/config/mount/>.

## Decide

| You need | Do this |
|----------|---------|
| The guest to see one host directory | `--mount-only "<dir>:w"` (drop `:w` for read-only) |
| No host filesystem visible at all | `--mount-none` (removes Lima's mounts only; see the WSL2 caveat below) |
| To add a directory to the default mounts | `--mount "<dir>:w"` |
| A one-off file transfer | `limactl copy` |
| A guest file pulled to the host on every start | a `copyToHost` rule in the template |
| The guest to work on a copy, synced back on exit | `limactl shell --sync <DIR> <INSTANCE>`, Lima 2.1+ |
| More disk, not more sharing | `limactl disk create` and `additionalDisks` |

`--mount-only` replaces the template's mounts. `--mount` adds to them. Mounting less is the
safer default: a VM that cannot see a directory cannot damage it.

`--mount-none` is not a filesystem boundary on the `wsl2` driver. WSL's own drive automount keeps
every host drive visible under `/mnt` there, whatever Lima's mount list says, and Lima does not
disable it. On other drivers the mount list is the whole story.

## Mount types

| Type | Where | Notes |
|------|-------|-------|
| `reverse-sshfs` | all drivers | the portable default; a compromised guest process can reach unmounted host directories through the host SFTP server |
| `virtiofs` | vz on macOS; Linux (experimental) | fastest where available. On Linux it needs the Rust `virtiofsd`; the QEMU-packaged `qemu-virtiofsd` will not work |
| `9p` | qemu | |

Select with `--mount-type`. Availability depends on the driver and host OS, so let Lima choose
unless there is a measured reason not to.

## Do

```bash
# only this project, writable, nothing else
limactl start --tty=false --name dev --mount-only "$PWD:w" template:default

# no host filesystem at all
limactl start --tty=false --name sealed --mount-none template:default

# copy files; prefix a guest path with the instance name and a colon
limactl copy ./config.yaml dev:/tmp/config.yaml
limactl copy dev:/etc/os-release .
limactl copy -r ./src dev:/tmp/src
```

`limactl copy` picks rsync when both sides have it and falls back to scp. Force one with
`--backend=rsync` or `--backend=scp`.

To pull a guest file back on every start, a template uses `copyToHost`, which takes four
fields:

```yaml
copyToHost:
- guest: "/etc/rancher/k3s/k3s.yaml"
  host: "{{.Dir}}/copied-from-guest/kubeconfig.yaml"
  skipIfMissing: true     # no error when the guest file is absent
  deleteOnStop: true      # remove the host copy when the instance stops
```

`skipIfMissing` is why a failed cluster leaves no file and no error, and `deleteOnStop` is
why a path that worked before a restart can be gone afterwards. Both are set on the bundled
Kubernetes templates.

## Working on a copy with `--sync`

`limactl shell --sync` copies a host directory into the guest, runs the command there, and
syncs the result back after the command exits successfully. It needs rsync on both sides, and the
instance must have been created with `--mount-none`.

```bash
limactl start --tty=false --name sealed --mount-none template:default
limactl shell --sync . sealed make build
```

**The accept/discard prompt only appears when stdout is a terminal.** Piping or redirecting
the output loses it, as do `--tty=false` and `-y`, and the sync back then happens
unconditionally. Treat `--sync` as an automatic apply whenever output is not a terminal: start
from a git-clean tree and review with `git diff` afterwards. Answering **No** at the prompt
also deletes the guest copy, so view or copy the work out first if it matters.

**Requires Lima >= 2.3.** Exclude paths with `--sync-exclude` (repeatable) or a
`.limasyncignore` file in the synced directory. Flags come before the instance name; anything
after it is the guest command, and on an older host `--sync-exclude` fails as an unknown flag
rather than reaching the guest.

```bash
limactl shell --sync . --sync-exclude .git sealed make build
```

## Additional disks

A Lima disk is a separate volume with its own lifetime. It outlives the instance it is
attached to, which makes it the right place for a dataset or a build cache you do not want
to rebuild. It is not a mount, and it shares nothing with the host filesystem.

Create it, then attach it to a **stopped** instance. There is no CLI flag for attaching, so
the attach step is a `--set` on the instance config:

```bash
limactl disk create scratch --size 200GiB
limactl stop dev
limactl edit dev --tty=false --set '.additionalDisks += [{"name":"scratch"}]'
limactl start dev
```

Lima partitions, formats and mounts it for you on first use. The defaults are `format: true`,
`fsType: ext4`, `mount: true`, and a mount point of **`/mnt/lima-<name>`**. Override the
location with `mountPoint`, or set `mount: false` to attach the raw device and handle it
yourself. The partition is labelled `/dev/disk/by-partlabel/lima-<name>`.

A disk may be attached to only one instance at a time; `limactl disk list` shows which one
holds it, and `limactl disk unlock` releases a stale claim. **Starting an instance that names
a disk which does not exist fails**, since the disk is not created implicitly.

```bash
limactl disk list
limactl shell dev df -h /mnt/lima-scratch
```

## Verify

```bash
limactl shell dev ls -la /path/in/guest
limactl shell dev sh -c 'touch /path/in/guest/.probe && rm /path/in/guest/.probe'
```

The second line is the only reliable test that a mount is writable.

## When it breaks

| Signature | Cause | Fix |
|-----------|-------|-----|
| The mount point is empty in the guest | the host path does not exist, or the mount failed | check `ha.stderr.log`; the host path must exist before start |
| `Read-only file system` | the mount has no `:w` | recreate with `--mount-only "<dir>:w"` |
| Mounts are missing entirely | `--plain` or `--mount-none` | use neither, or add `--mount` |
| Builds are much slower than on the host | filesystem translation overhead | keep the build tree inside the guest, or try `--mount-type=virtiofs` |
| `--sync` refuses to run | the instance has host mounts, or rsync is missing | recreate with `--mount-none`; install rsync on both sides |
| `--sync` says "cd to a deeper directory" | the host directory is fewer than four path segments deep | work from a deeper path |
| `--sync` is rejected on a wsl2 instance | `/mnt` already exposes the host there | use mounts instead |
| Overlapping mounts behave oddly | overlapping mount paths are not supported | mount non-overlapping directories |
