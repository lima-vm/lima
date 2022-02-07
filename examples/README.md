# Lima examples

⭐ = ["Tier 1"](#tier-1)

Default: [`default.yaml`](../pkg/limayaml/default.yaml) (⭐Ubuntu, with containerd/nerdctl)

Distro:
- [`almalinux.yaml`](./almalinux.yaml): AlmaLinux
- [`alpine.yaml`](./alpine.yaml): ⭐Alpine Linux
- [`archlinux.yaml`](./archlinux.yaml): ⭐Arch Linux
- [`debian.yaml`](./debian.yaml): ⭐Debian GNU/Linux
- [`fedora.yaml`](./fedora.yaml): ⭐Fedora
- [`opensuse.yaml`](./opensuse.yaml): ⭐openSUSE Leap
- [`rocky.yaml`](./rocky.yaml): Rocky Linux
- [`ubuntu.yaml`](./ubuntu.yaml): Ubuntu (same as `default.yaml` but without extra YAML lines)
- [`ubuntu-lts.yaml`](./ubuntu-lts.yaml): Ubuntu LTS (same as `ubuntu.yaml` but pinned to an LTS version)

Container engines:
- [`docker.yaml`](./docker.yaml): Docker
- [`podman.yaml`](./podman.yaml): Podman
- [`singularity.yaml`](./singularity.yaml): Singularity
- LXD is installed in the default Ubuntu template, so there is no `lxd.yaml`

Container orchestration:
- [`k3s.yaml`](./k3s.yaml): Kubernetes via k3s
- [`k8s.yaml`](./k8s.yaml): Kubernetes via kubeadm
- [`nomad.yaml`](./nomad.yaml): Nomad
- [`faasd.yaml`](./faasd.yaml): [Faasd](https://docs.openfaas.com/deployment/faasd/)

Others:
- [`vmnet.yaml`](./vmnet.yaml): ⭐enable [`vmnet.framework`](../docs/network.md)

## Tier 1

The "Tier 1" yamls (marked with ⭐) are regularly tested on the CI.

Other yamls are tested only occasionally and manually.

## Usage
Run `limactl start fedora.yaml` to create a Lima instance named "fedora".

To open a shell, run `limactl shell fedora bash` or `LIMA_INSTANCE=fedora lima bash`.
