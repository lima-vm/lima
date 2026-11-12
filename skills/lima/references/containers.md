# Containers

Running containers inside a Lima VM and, when needed, driving them from the host. Reference:
<https://lima-vm.io/docs/examples/containers/>.

Runtimes are peers here. containerd is what a stock `limactl start` already gives you; Podman
and Docker are template choices, not fallbacks.

## Decide

| The task needs | Runtime | Template |
|----------------|---------|----------|
| Containers, driven from inside the VM | containerd with nerdctl | none, `template:default` has it |
| A daemonless runtime with a host-side client | Podman | `template:podman` |
| Docker API compatibility for existing host tooling | Docker | `template:docker` |
| Privileged ports, or images that assume root | add `-rootful` to the Podman or Docker template |
| Image building only | BuildKit | `template:buildkit` |
| Single-file images, HPC conventions | Apptainer | `template:apptainer` |
| No container runtime at all | | `limactl start --containerd=none` |

Use a named instance per project. That, not a flag, is what keeps one project's daemon away
from another's and away from the host's.

The runtime templates pick their own base image, which is Ubuntu for `docker` and `podman`.
There is no flag that makes `template:docker` run on Fedora. If a task needs a specific distro
*and* a specific runtime, base a template on the distro image and install the runtime with a
`provision:` script. See [templates.md](templates.md), which covers both. Do not assume
`template:docker` gives you the distro the user asked for.

A hand-written runtime template also has to publish the socket itself, or there will be
nothing at `{{.Dir}}/sock/` for the host client to talk to. That is a `portForwards` entry,
and the guest path differs between rootless and rootful:

```yaml
portForwards:
# rootless docker
- guestSocket: "/run/user/{{.UID}}/docker.sock"
  hostSocket: "{{.Dir}}/sock/docker.sock"
# rootless podman would be /run/user/{{.UID}}/podman/podman.sock
# rootful would be /var/run/docker.sock
```

## Do

containerd, no host client needed:

```bash
limactl start -y --name proj template:default
limactl shell proj nerdctl run -d --name web -p 127.0.0.1:8080:80 nginx:alpine
```

The `lima nerdctl ...` and `nerdctl.lima ...` wrappers are shorter but target `$LIMA_INSTANCE`,
which defaults to `default`. Set `LIMA_INSTANCE=proj` before using them, or stay with
`limactl shell proj`.

Podman, isolated per project. The same socket also answers `DOCKER_HOST`, so host tooling
that speaks the Docker API works against it:

```bash
limactl start -y --name proj template:podman
export CONTAINER_HOST=$(limactl list proj --format 'unix://{{.Dir}}/sock/podman.sock')
podman --remote run -d --name web -p 127.0.0.1:8080:80 docker.io/library/nginx:alpine
```

Docker, isolated per project:

```bash
limactl start -y --name proj template:docker
export DOCKER_HOST=$(limactl list proj --format 'unix://{{.Dir}}/sock/docker.sock')
docker run -d --name web -p 127.0.0.1:8080:80 nginx:alpine
```

Publish container ports to `127.0.0.1` inside the guest, as above. Lima forwards guest
localhost to host localhost; a container bound to `0.0.0.0` inside the guest is not
automatically reachable from the host. See [networking.md](networking.md).

`nerdctl.lima`, `docker.lima`, `podman.lima` and `apptainer.lima` are wrapper commands
installed alongside `limactl` that target the instance named by `$LIMA_INSTANCE`.

## Verify

The hostname must be the guest, not the host:

```bash
docker info --format '{{.Name}}'
podman --remote info --format '{{.Host.Hostname}}'
```

## When it breaks

| Signature | Cause | Fix |
|-----------|-------|-----|
| `Cannot connect to the Docker daemon` | the instance is stopped | `limactl list <inst> --format json`, check `.status` |
| The socket path does not exist | the template has no such runtime, or the instance never finished provisioning | recreate from the right template; check `ha.stderr.log` |
| Host tooling hits the host daemon instead | `DOCKER_HOST` or `CONTAINER_HOST` is not exported in that shell | re-export; verify with the hostname check above |
| A rootless container cannot bind a port below 1024 | expected under rootless | use the `-rootful` template |
| A published port is not reachable from the host | the container bound `0.0.0.0` inside the guest | publish to `127.0.0.1:<port>` instead |

## Cleanup

```bash
limactl stop proj
limactl delete -y proj
```

Deleting the instance removes its images, volumes and socket. There is nothing to clean on the
host.
