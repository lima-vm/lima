# Networking

Reaching a guest port from the host, and a host service from the guest. Reference:
<https://lima-vm.io/docs/config/port/> and <https://lima-vm.io/docs/config/network/>.

## Decide

| You need | Do this |
|----------|---------|
| A guest port on the host | bind `127.0.0.1:<port>` in the guest; Lima forwards it automatically |
| A fixed mapping, known before the service starts | `--port-forward <host>:<guest>` at creation |
| A port forwarded even in `--plain` mode | a `portForwards` rule with `static: true` |
| The host to join the guest network | `limactl tunnel <inst>`, a SOCKS proxy |
| The guest to have its own IP on the host network | an additional network, `--network lima:shared` or `--network vzNAT` |
| A host service reachable from inside the guest | bind it on the host loopback and use `host.lima.internal` from the guest |

The default rule is `guestIP: 127.0.0.1`, and it **also matches `0.0.0.0`, `::` and `::1`**, so
a service bound to any of those is forwarded. What is *not* matched is a listener bound to one
specific non-loopback address, such as the guest's own `192.168.5.15`. Bind loopback or all
interfaces, not a single interface.

## Do

```bash
# fixed mapping at creation time
limactl start --tty=false --name dev --port-forward 8080:80 template:default

# follow new forwarding events as they happen (blocks; it does not show current state)
limactl watch dev

# what has been forwarded so far, without blocking
grep -i 'forward' "$(limactl list dev --format '{{.Dir}}')/ha.stderr.log" | tail -20

# SOCKS proxy onto the guest network
limactl tunnel --socks-port 1080 dev
```

From inside the guest, the host is `192.168.5.2` under Lima's default user-mode networking,
and that address also resolves as **`host.lima.internal`**. Prefer the name. A host service
listening on `127.0.0.1:5432` is reachable at `host.lima.internal:5432`.

`limactl tunnel` does not stay in the foreground. It forks the SSH process into the
background and returns, then prints the environment line to use, of the form
`ALL_PROXY=socks5h://127.0.0.1:<port>`. Use `socks5h`, not `socks5`, so that names resolve
inside the guest network. It also reports that the instance is reachable from the host as
`<name>.internal`.

### Adding a forward to an instance that already exists

`--port-forward` is a creation-time flag, and `limactl edit` has no equivalent. Edit the stored
config, then restart so the hostagent picks it up. `limactl edit` refuses a running instance, so
use `restart --set`, which applies the expression and restarts in one step:

```bash
limactl restart <inst> --set '.portForwards += [{"guestPort":8080,"hostPort":18080}]'
```

To edit a stopped instance instead, use `limactl edit <inst> --tty=false --set '...'` and then
`limactl start <inst>`.

A `--plain` instance forwards nothing dynamically, so its rules need `static`:

```bash
limactl restart <inst> --set '.portForwards += [{"guestPort":8080,"hostPort":18080,"static":true}]'
```

Prefer binding the guest service to `127.0.0.1` and letting automatic forwarding handle it.
Reach for a rule when the guest port is fixed, when you need a different host port, or when the
instance is `--plain`.

`--port-forward` also takes the `static` suffix at creation time, so a locked-down instance
with exactly one forward is a single command rather than create-then-edit-then-restart:

```bash
limactl start --tty=false --name sectest --plain --port-forward 8080:80,static=true template:default
```

### Forwarding nothing except one port

Rules are evaluated in order and the first match wins, so a deny-by-default setup is an allow
rule followed by a catch-all `ignore` rule. The order matters: put the allow rule first.

The `ignore` rule needs an unspecified `guestIP` of its own. A rule that omits `guestIP` defaults
to `127.0.0.1`, and a rule with a specified guest IP is skipped for listeners bound to `0.0.0.0`
or `::`, which the hostagent's appended default rule then forwards. So the obvious rule silently
fails to deny anything bound to all interfaces:

```bash
limactl restart <inst> --set \
  '.portForwards = [{"guestPort":80,"hostPort":8080},{"guestPortRange":[1,65535],"ignore":true,"guestIP":"0.0.0.0","guestIPMustBeZero":false}]'
```

Read it back with `limactl list <inst> --format json | jq '.config.portForwards'` before trusting
it.

Prefer this to `--plain` when the goal is only to control forwarding. `--plain` also disables
mounts, containerd and the guest agent, which changes how the instance behaves in ways a
security test usually does not want.

Lima still listens on a host loopback port for its own SSH connection whatever the rules say,
so "no host-facing ports at all" and `limactl shell` are mutually exclusive. Check what is
actually listening with `limactl list <inst> --format '{{.SSHLocalPort}}'`.

## Port forwarder implementations

Lima has two. The default has changed between releases, so check rather than assume.

| Forwarder | Select with | Notes |
|-----------|-------------|-------|
| gRPC | `LIMA_SSH_PORT_FORWARDER=false` | supports UDP |
| SSH | `LIMA_SSH_PORT_FORWARDER=true` | no UDP; faster when AF_VSOCK is available |

See <https://lima-vm.io/docs/config/port/> for which is default in the installed version.

## Verify

```bash
limactl shell dev ss -ltn                 # what the service is actually bound to
limactl shell dev curl -fsS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:80
curl -fsS -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8080
```

`ss -ltn` shows the bind address. `127.0.0.1:8080`, `0.0.0.0:8080` and `[::]:8080` are all
forwarded by the default rule; a specific address such as `192.168.5.15:8080` is not.

The in-guest `curl` proves the service is up; the host `curl` proves forwarding works. Running
only the host one cannot tell those two failures apart.

## When it breaks

| Signature | Cause | Fix |
|-----------|-------|-----|
| The port works in the guest, not on the host | the service bound one specific non-loopback address | bind `127.0.0.1` or `0.0.0.0` in the guest |
| Nothing is forwarded at all | `--plain` disables dynamic forwarding | add a `portForwards` rule with `static: true`, or do not use `--plain` |
| `guest agent does not seem to be running` | the guestagent reports listening ports; without it nothing is discovered | [troubleshooting.md](troubleshooting.md) |
| Forwards die after an in-guest reboot | host-side forwarding is not re-established | `limactl restart <inst>` from the host |
| A UDP service is not forwarded | the SSH forwarder does not do UDP | set `LIMA_SSH_PORT_FORWARDER=false` and restart |
| The host port is already taken | another process or another instance | pick another host port; `--port-forward 18080:80` |
