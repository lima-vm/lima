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
| A host service reachable from inside the guest | bind it on the host loopback and use `192.168.5.2` from the guest |

Automatic forwarding only covers guest **localhost**. A service bound to `0.0.0.0` inside the
guest is not forwarded. This is the single most common surprise.

## Do

```bash
# fixed mapping at creation time
limactl start -y --name dev --port-forward 8080:80 template:default

# what is forwarded right now
limactl watch dev

# SOCKS proxy onto the guest network
limactl tunnel --socks-port 1080 dev
```

From inside the guest, `192.168.5.2` is the host under Lima's default user-mode networking, so
a host service listening on `127.0.0.1:5432` is reachable at `192.168.5.2:5432`.

### Adding a forward to an instance that already exists

`--port-forward` is a creation-time flag, and `limactl edit` has no equivalent. Edit the stored
config with a yq expression, then restart so the hostagent picks it up:

```bash
limactl edit <inst> --tty=false --set '.portForwards += [{"guestPort":8080,"hostPort":18080}]'
limactl restart <inst>
```

A `--plain` instance forwards nothing dynamically, so its rules need `static`:

```bash
limactl edit <inst> --tty=false \
  --set '.portForwards += [{"guestPort":8080,"hostPort":18080,"static":true}]'
limactl restart <inst>
```

Prefer binding the guest service to `127.0.0.1` and letting automatic forwarding handle it.
Reach for a rule when the guest port is fixed, when you need a different host port, or when the
instance is `--plain`.

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

`ss -ltn` settles the usual question before guessing: an address of `0.0.0.0:8080` is not
forwarded, `127.0.0.1:8080` is.

The in-guest `curl` proves the service is up; the host `curl` proves forwarding works. Running
only the host one cannot tell those two failures apart.

## When it breaks

| Signature | Cause | Fix |
|-----------|-------|-----|
| The port works in the guest, not on the host | the service bound `0.0.0.0` | bind `127.0.0.1` in the guest |
| Nothing is forwarded at all | `--plain` disables dynamic forwarding | add a `portForwards` rule with `static: true`, or do not use `--plain` |
| `guest agent does not seem to be running` | the guestagent reports listening ports; without it nothing is discovered | [troubleshooting.md](troubleshooting.md) |
| Forwards die after an in-guest reboot | host-side forwarding is not re-established | `limactl restart <inst>` from the host |
| A UDP service is not forwarded | the SSH forwarder does not do UDP | set `LIMA_SSH_PORT_FORWARDER=false` and restart |
| The host port is already taken | another process or another instance | pick another host port; `--port-forward 18080:80` |
