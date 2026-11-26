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
| Images that need root inside the container, or a daemon that must run as root | add `-rootful` to the Podman or Docker template |
| Image building only | BuildKit | `template:buildkit` |
| Single-file images, HPC conventions | Apptainer | `template:apptainer` |
| No container runtime at all | | `limactl start --containerd=none` |

Use a named instance per project. That, not a flag, is what keeps one project's daemon away
from another's and away from the host's.

The runtime templates pick their own base image, and they do not all pick the same one:
`template:docker` is Ubuntu, `template:podman` is Fedora. Check before assuming, with
`limactl template yq template:<name> '.images[].location'` (`.base` is already resolved away by
the time the expression is evaluated).

Podman's socket also answers the Docker API, so **a Fedora guest driven by Docker-API host
tooling is `template:podman`**, not a hand-built template. There is no flag that makes
`template:docker` run on Fedora. When no stock template has the distro and runtime a task
needs, base one on the distro image and install the runtime with a `provision:` script. See
[templates.md](templates.md), which covers both.

A hand-written runtime template also has to publish the socket itself, or there will be
nothing at `{{.Dir}}/sock/` for the host client to talk to. That is a `portForwards` entry,
and the guest path differs between rootless and rootful:

```yaml
portForwards:
# rootless docker
- guestSocket: "/run/user/{{.UID}}/docker.sock"
  hostSocket: "{{.Dir}}/sock/docker.sock"
```

The guest path differs per runtime and per rootless or rootful, and the two rootful paths are
not the same shape:

| Runtime | Rootless guest socket | Rootful guest socket |
|---------|-----------------------|----------------------|
| Docker | `/run/user/{{.UID}}/docker.sock` | `/var/run/docker.sock` |
| Podman | `/run/user/{{.UID}}/podman/podman.sock` | `/run/podman/podman.sock` |
| BuildKit | `/run/user/{{.UID}}/buildkit-default/buildkitd.sock` | |

## Do

containerd, no host client needed:

```bash
limactl start --tty=false --name proj template:default
limactl shell proj nerdctl run -d --name web -p 127.0.0.1:8080:80 nginx:alpine
```

The `lima nerdctl ...` and `nerdctl.lima ...` wrappers are shorter but target `$LIMA_INSTANCE`,
which defaults to `default`. Set `LIMA_INSTANCE=proj` before using them, or stay with
`limactl shell proj`.

Podman, isolated per project. The same socket also answers `DOCKER_HOST`, so host tooling
that speaks the Docker API works against it:

```bash
limactl start --tty=false --name proj template:podman
export CONTAINER_HOST=$(limactl list proj --format 'unix://{{.Dir}}/sock/podman.sock')
podman --remote run -d --name web -p 127.0.0.1:8080:80 docker.io/library/nginx:alpine
```

Docker, isolated per project:

```bash
limactl start --tty=false --name proj template:docker
export DOCKER_HOST=$(limactl list proj --format 'unix://{{.Dir}}/sock/docker.sock')
docker run -d --name web -p 127.0.0.1:8080:80 nginx:alpine
```

BuildKit, for building images with no daemon to manage. The template prints the export line
itself on start:

```bash
limactl start --tty=false --name build template:buildkit
export BUILDKIT_HOST=$(limactl list build --format 'unix://{{.Dir}}/sock/buildkitd.sock')
buildctl debug workers
# build an image from a Dockerfile in the current directory, into an OCI tar
buildctl build --frontend dockerfile.v0 --local context=. --local dockerfile=. \
  --output type=oci,dest=image.tar
```

The image appears on the host through the forwarded socket; nothing has to be copied out of the
guest. Frontend and output flags vary by task, so `buildctl build --help` is the authority for
the installed version.

Apptainer, for HPC-style single-file images. `-u` runs unprivileged and `-B` binds a host
path that the guest already has mounted:

```bash
limactl start --tty=false --name hpc template:apptainer
limactl shell hpc apptainer run -u -B $HOME:$HOME docker://alpine
```

Publish container ports on the guest, as above. The default forward rule matches `127.0.0.1`,
`0.0.0.0` and `::`, so either bind works; what is not forwarded is a single specific
non-loopback address. See [networking.md](networking.md).

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
| A low port is refused inside the guest | check `limactl shell <inst> sysctl net.ipv4.ip_unprivileged_port_start`. Stock Linux guests set it to `0`, so ports below 1024 work rootless; only a guest that leaves it at 1024 needs a rootful runtime |
| A published port is not reachable from the host | the port was published inside the container only, not onto the guest | check `limactl shell <inst> ss -ltn` shows it on the guest |

## Cleanup

```bash
limactl stop proj
limactl delete proj   # immediate, with no confirmation prompt
```

Deleting the instance removes its images, volumes and socket. There is nothing to clean on the
host.
