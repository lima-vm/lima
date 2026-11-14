# Troubleshooting

Finding out which of the three processes failed. Reference:
<https://lima-vm.io/docs/dev/internals/>.

## Decide

Start from the symptom, not from the top.

| Symptom | Look at |
|---------|---------|
| `limactl start` never returns | `serial*.log` for a boot failure, then `ha.stderr.log` |
| The instance is `Running` but a forwarded port is dead | `ha.stderr.log`, for guestagent errors |
| A mount is missing or empty inside the guest | `ha.stderr.log` |
| A provisioning script did not run | `/var/log/cloud-init-output.log` inside the guest |
| The command fails before the VM starts at all | re-run with `--debug` |

## Do

```bash
# where this instance keeps its state
DIR=$(limactl list <inst> --format '{{.Dir}}')

# status, plus any errors Lima already knows about
limactl list <inst> --format json | jq '{status, vmType, errors, message}'

# hostagent: port forwarding, mounts, guestagent connection
tail -n 50 "$DIR/ha.stderr.log"

# guest console: kernel panics, cloud-init failures, disk problems
tail -n 50 "$DIR"/serial*.log

# host configuration: version, drivers, template and plugin paths
limactl info

# watch lifecycle and port-forwarding events as they happen
limactl watch <inst>

# anything still unexplained
limactl --debug start <inst>
```

## When it breaks

| Signature | Cause | Fix |
|-----------|-------|-----|
| `guest agent does not seem to be running; port forwards will not work` | the guestagent is not reachable over its socket | check `ha.stderr.log`; on a recent Fedora guest this is the SELinux case below |
| Forwarded ports stop responding after an upgrade or reboot inside the guest | rebooting from inside does not re-establish host port forwarding | restart from the host: `limactl restart <inst>`. Do not reboot in-guest in automation |
| A forwarded port never appears | the service bound `0.0.0.0`, or `--plain` disabled dynamic forwarding | bind `127.0.0.1` in the guest; see [networking.md](networking.md) |
| The guest boots but `limactl shell` times out | SSH is not up, or the host key changed after a rebuild | `limactl --debug shell <inst>`; delete and recreate if the instance was rebuilt in place |
| A mount is empty | the mount type is unsupported for this driver, or the host path does not exist | see [mounts.md](mounts.md) |
| Disk full inside the guest | the disk was sized at creation | `limactl edit <inst> --disk <size>` (grows only; `limactl disk resize` is for additional disks, not the primary one) |

## SELinux blocks the guestagent socket

Fedora's `selinux-policy` 44.7 made the `sshd_session_t` domain enforcing. It had been
permissive, and the policy does not allow it to connect to the Unix sockets Lima forwards, so
`/run/lima-guestagent.sock` is denied. The symptom is `guest agent does not seem to be
running; port forwards will not work`, with `error reading server preface: connection reset`
in `ha.stderr.log`. Upstream issue: <https://github.com/lima-vm/lima/issues/5572>.

**Lima 2.3 and later fix this themselves.** A boot script installs an SELinux module named
`lima-sshd-session` on first boot, but only when it detects the domain is enforcing. Check it
landed:

```bash
limactl shell <inst> semodule -l | grep lima-sshd-session
```

On Lima 2.2 or older the guest has no such script, so install the module by hand in the guest:

```bash
cat >/tmp/lima-sshd-session.cil <<'EOF'
(allow sshd_session_t var_run_t (sock_file (write)))
(allow sshd_session_t unconfined_service_t (unix_stream_socket (connectto)))
(optional lima_sshd_session_container
	(allow sshd_session_t container_runtime_t (unix_stream_socket (connectto))))
EOF
sudo semodule -i /tmp/lima-sshd-session.cil
```

Then `limactl restart <inst>` from the host. The `optional` block covers the podman and docker
sockets and is skipped when `container-selinux` is not installed. Upgrading Lima is the better
fix; bake the module into the image only if you are pinned to an older release.

## Last resorts

Both destroy state. Confirm with the user first.

```bash
limactl factory-reset <inst>   # wipe the instance, keep its configuration
limactl prune                  # remove cached images and unused objects
```

## Reporting upstream

Include the output of `limactl info`, the failing command with `--debug`, and the relevant
tail of `ha.stderr.log`. Redact host names and absolute paths that are not needed to reproduce.
