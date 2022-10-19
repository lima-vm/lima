# Lima examples

⭐ = ["Tier 1"](#tier-1)

Default: [`default.yaml`](./default.yaml) (⭐Ubuntu, with containerd/nerdctl)

Distro:
- [`almalinux-8.yaml`](./almalinux-8.yaml): AlmaLinux 8
- [`almalinux-9.yaml`](./almalinux-9.yaml), `almalinux.yaml`: AlmaLinux 9
- [`alpine.yaml`](./alpine.yaml): ⭐Alpine Linux
- [`archlinux.yaml`](./archlinux.yaml): ⭐Arch Linux
- [`centos-stream-8.yaml`](./centos-stream-8.yaml): CentOS Stream 8
- [`centos-stream-9.yaml`](./centos-stream-9.yaml), `centos-stream.yaml`: CentOS Stream 9
- [`debian.yaml`](./debian.yaml): ⭐Debian GNU/Linux
- [`fedora.yaml`](./fedora.yaml): ⭐Fedora
- [`opensuse.yaml`](./opensuse.yaml): ⭐openSUSE Leap
- [`oraclelinux-8.yaml`](./oraclelinux-8.yaml): Oracle Linux 8
- [`oraclelinux-9.yaml`](./oraclelinux-9.yaml), `oraclelinux.yaml`: Oracle Linux 9
- [`rocky-8.yaml`](./rocky-8.yaml): Rocky Linux 8
- [`rocky-9.yaml`](./rocky-9.yaml), `rocky.yaml`: Rocky Linux 9
- [`ubuntu.yaml`](./ubuntu.yaml): Ubuntu (same as `default.yaml` but without extra YAML lines)
- [`ubuntu-lts.yaml`](./ubuntu-lts.yaml): Ubuntu LTS (same as `ubuntu.yaml` but pinned to an LTS version)
- [`deprecated/centos-7.yaml`](./deprecated/centos-7.yaml): [deprecated] CentOS Linux 7
- [`experimental/opensuse-tumbleweed.yaml`](experimental/opensuse-tumbleweed.yaml): [experimental] openSUSE Tumbleweed

Container engines:
- [`apptainer.yaml`](./apptainer.yaml): Apptainer
- [`apptainer-rootful.yaml`](./apptainer-rootful.yaml): Apptainer (rootful)
- [`docker.yaml`](./docker.yaml): Docker
- [`docker-rootful.yaml`](./docker-rootful.yaml): Docker (rootful)
- [`podman.yaml`](./podman.yaml): Podman
- [`podman-rootful.yaml`](./podman-rootful.yaml): Podman (rootful)
- LXD is installed in the default Ubuntu template, so there is no `lxd.yaml`

Container image builders:
- [`buildkit.yaml`](./buildkit.yaml): BuildKit

Container orchestration:
- [`faasd.yaml`](./faasd.yaml): [Faasd](https://docs.openfaas.com/deployment/faasd/)
- [`k3s.yaml`](./k3s.yaml): Kubernetes via k3s
- [`k8s.yaml`](./k8s.yaml): Kubernetes via kubeadm
- [`nomad.yaml`](./nomad.yaml): Nomad

Optional feature enablers:
- [`vmnet.yaml`](./vmnet.yaml): ⭐enable [`vmnet.framework`](../docs/network.md)
- [`experimental/9p.yaml`](experimental/9p.yaml): [experimental] use 9p mount type
- [`experimental/riscv64.yaml`](experimental/riscv64.yaml): [experimental] RISC-V

Lost+found:
- ~`centos.yaml`~: Removed in Lima v0.8.0, as CentOS 8 reached [EOL](https://www.centos.org/centos-linux-eol/).
  Replaced by [`almalinux.yaml`](./almalinux.yaml), [`centos-stream.yaml`](./centos-stream.yaml), [`oraclelinux.yaml`](./oraclelinux.yaml),
  and [`rocky.yaml`](./rocky.yaml).
- ~`singularity.yaml`~: Moved to [`apptainer-rootful.yaml`](./apptainer-rootful.yaml) in Lima v0.13.0, as Singularity was renamed to Apptainer.
- ~`experimental/apptainer.yaml`~: Moved to [`apptainer.yaml`](./apptainer.yaml) in Lima v0.13.0.
- ~`experimental/{almalinux,centos-stream-9,oraclelinux,rocky}-9.yaml`~: Moved to [`almalinux-9.yaml`](./almalinux.yaml), [`centos-stream-9.yaml`](./centos-stream-9.yaml),
  [`oraclelinux-9.yaml`](./oraclelinux-9.yaml), and [`rocky-9.yaml`](./rocky-9.yaml) in Lima v0.13.0.

## Tier 1

The "Tier 1" yamls (marked with ⭐) are regularly tested on the CI.

Other yamls are tested only occasionally and manually.

## Usage
Run `limactl start fedora.yaml` to create a Lima instance named "fedora".

To open a shell, run `limactl shell fedora bash` or `LIMA_INSTANCE=fedora lima bash`.
