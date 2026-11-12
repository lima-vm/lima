# Mounts and files

Sharing host directories with a guest, and moving files either way. Reference:
<https://lima-vm.io/docs/config/mount/>.

## Decide

| You need | Do this |
|----------|---------|
| The guest to see one host directory | `--mount-only "<dir>:w"` (drop `:w` for read-only) |
| No host filesystem visible at all | `--mount-none` |
| To add a directory to the default mounts | `--mount "<dir>:w"` |
| A one-off file transfer | `limactl copy` |
| A guest file pulled to the host on every start | a `copyToHost` rule in the template |
| The guest to work on a copy, synced back on exit | `limactl shell --sync`, Lima 2.1+ |
| More disk, not more sharing | `limactl disk create` and `additionalDisks` |

`--mount-only` replaces the template's mounts. `--mount` adds to them. Mounting less is the
safer default: a VM that cannot see a directory cannot damage it.

## Mount types

| Type | Where | Notes |
|------|-------|-------|
| `reverse-sshfs` | all drivers | the portable default |
| `virtiofs` | vz on macOS, and Linux | fastest where available |
| `9p` | qemu | |

Select with `--mount-type`. Availability depends on the driver and host OS, so let Lima choose
unless there is a measured reason not to.

## Do

```bash
# only this project, writable, nothing else
limactl start -y --name dev --mount-only "$PWD:w" template:default

# no host filesystem at all
limactl start -y --name sealed --mount-none template:default

# copy files; prefix a guest path with the instance name and a colon
limactl copy ./config.yaml dev:/tmp/config.yaml
limactl copy dev:/etc/os-release .
limactl copy -r ./src dev:/tmp/src
```

`limactl copy` picks rsync when both sides have it and falls back to scp. Force one with
`--backend=rsync` or `--backend=scp`.

## Working on a copy with `--sync`

`limactl shell --sync` copies a host directory into the guest, runs the command there, and
syncs the result back on exit. It needs rsync on both sides, and the instance must have been
created with `--mount-none`.

```bash
limactl start -y --name sealed --mount-none template:default
limactl shell --sync . sealed make build
```

**The accept/discard prompt only appears when stdout is a terminal.** Piping or redirecting
the output loses it, as do `--tty=false` and `-y`, and the sync back then happens
unconditionally. Treat `--sync` as an automatic apply whenever output is not a terminal: start
from a git-clean tree and review with `git diff` afterwards. Answering **No** at the prompt
also deletes the guest copy, so view or copy the work out first if it matters.

As of Lima 2.3, exclude paths with `--sync-exclude` (repeatable) or a `.limasyncignore` file
in the synced directory. Both flags must come before the instance name:

```bash
limactl shell --sync . --sync-exclude .git sealed make build
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
