# Lima examples

⭐ = ["Tier 1"](#tier-1)

Default: [`default.yaml`](./default.yaml) (⭐Ubuntu, with containerd/nerdctl)

Distro:
- [`almalinux.yaml`](./almalinux.yaml): AlmaLinux
- [`alpine.yaml`](./alpine.yaml): ⭐Alpine Linux
- [`archlinux.yaml`](./archlinux.yaml): ⭐Arch Linux
- [`centos-stream.yaml`](./centos-stream.yaml): CentOS Stream 8
- [`debian.yaml`](./debian.yaml): ⭐Debian GNU/Linux
- [`fedora.yaml`](./fedora.yaml): ⭐Fedora
- [`opensuse.yaml`](./opensuse.yaml): ⭐openSUSE Leap
- [`oraclelinux.yaml`](./oraclelinux.yaml): Oracle Linux
- [`rocky.yaml`](./rocky.yaml): Rocky Linux
- [`ubuntu.yaml`](./ubuntu.yaml): Ubuntu (same as `default.yaml` but without extra YAML lines)
- [`ubuntu-lts.yaml`](./ubuntu-lts.yaml): Ubuntu LTS (same as `ubuntu.yaml` but pinned to an LTS version)

Container engines:
- [`docker.yaml`](./docker.yaml): Docker
- [`docker-rootful.yaml`](./docker-rootful.yaml): Docker (rootful)
- [`podman.yaml`](./podman.yaml): Podman
- [`podman-rootful.yaml`](./podman-rootful.yaml): Podman (rootful)
- [`apptainer.yaml`](./apptainer.yaml): Apptainer
- [`apptainer-rootful.yaml`](./apptainer-rootful.yaml): Apptainer (rootful)
- LXD is installed in the default Ubuntu template, so there is no `lxd.yaml`

Container image builders:
- [`buildkit.yaml`](./buildkit.yaml): BuildKit

Container orchestration:
- [`k3s.yaml`](./k3s.yaml): Kubernetes via k3s
- [`k8s.yaml`](./k8s.yaml): Kubernetes via kubeadm
- [`nomad.yaml`](./nomad.yaml): Nomad
- [`faasd.yaml`](./faasd.yaml): [Faasd](https://docs.openfaas.com/deployment/faasd/)

Others:
- [`vmnet.yaml`](./vmnet.yaml): ⭐enable [`vmnet.framework`](../docs/network.md)
- [`deprecated/centos-7.yaml`](./deprecated/centos-7.yaml): [deprecated] CentOS Linux 7
- [`experimental/almalinux-9.yaml`](experimental/almalinux-9.yaml): [experimental] AlmaLinux 9
- [`experimental/rocky-9.yaml`](experimental/rocky-9.yaml): [experimental] Rocky Linux 9
- [`experimental/oraclelinux-9.yaml`](experimental/oraclelinux-9.yaml): [experimental] Oracle Linux 9
- [`experimental/centos-stream-9.yaml`](experimental/centos-stream-9.yaml): [experimental] CentOS Stream 9
- [`experimental/opensuse-tumbleweed.yaml`](experimental/opensuse-tumbleweed.yaml): [experimental] openSUSE Tumbleweed
- [`experimental/9p.yaml`](experimental/9p.yaml): [experimental] use 9p mount type
- [`experimental/riscv64.yaml`](experimental/riscv64.yaml): [experimental] RISC-V

Lost+found:
- ~`centos.yaml`~: Removed in Lima v0.8.0, as CentOS 8 reached [EOL](https://www.centos.org/centos-linux-eol/).
  Replaced by [`rocky.yaml`](./rocky.yaml), [`almalinux.yaml`](./almalinux.yaml), [`oraclelinux.yaml`](./oraclelinux.yaml),
  and [`centos-stream.yaml`](./centos-stream.yaml).
- ~`singularity.yaml`~: Moved to [`apptainer-rootful.yaml`](./apptainer-rootful.yaml) in Lima v0.13.0, as Singularity was renamed to Apptainer.
- ~`experimental/apptainer.yaml`~: Moved to [`apptainer.yaml`](./apptainer.yaml) in Lima v0.13.0

## Tier 1

The "Tier 1" yamls (marked with ⭐) are regularly tested on the CI.

Other yamls are tested only occasionally and manually.

## Usage
Run `limactl start fedora.yaml` to create a Lima instance named "fedora".

To open a shell, run `limactl shell fedora bash` or `LIMA_INSTANCE=fedora lima bash`.
