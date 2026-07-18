## Quick usage

To create and start a new instance from the template `fedora`:
```bash
# Note: In Lima 1.x, template URLs included leading slashes, e.g. `template://fedora`.
limactl create template:fedora
limactl start fedora
```
or
```bash
limactl start template:fedora
# For the second time onward, just run `limactl start fedora`
```

To open a shell:
```bash
limactl shell fedora
```
or
```bash
export LIMA_INSTANCE=fedora
lima
```

## Template list

⭐ = ["Tier 1"](#tier)

☆ = ["Tier 2"](#tier)

### Default

- [`default`](./default.yaml) (⭐Ubuntu, with containerd/nerdctl)

### Linux distributions

- [`almalinux-8`](./almalinux-8.yaml): AlmaLinux 8
- [`almalinux-9`](./almalinux-9.yaml): AlmaLinux 9
- [`almalinux-10`](./almalinux-10.yaml), `almalinux`: AlmaLinux 10
- [`almalinux-kitten-10`](./almalinux-kitten-10.yaml), `almalinux-kitten`: AlmaLinux Kitten 10
- [`alpine`](./alpine.yaml): ☆Alpine Linux
- [`alpine-iso`](./alpine-iso.yaml): ☆Alpine Linux (ISO9660 image). Compatible with the `alpine` template used in Lima prior to v1.0.
- [`archlinux`](./archlinux.yaml): ☆Arch Linux
- [`centos-stream-9`](./centos-stream-9.yaml), `centos-stream`: CentOS Stream 9
- [`centos-stream-10`](./centos-stream-10.yaml): CentOS Stream 10
- [`debian-11`](./debian-11.yaml): Debian GNU/Linux 11 (Bullseye)
- [`debian-12`](./debian-12.yaml): Debian GNU/Linux 12 (Bookworm)
- [`debian-13`](./debian-13.yaml), `debian`: ⭐Debian GNU/Linux 13 (Trixie)
- [`fedora-41`](./fedora-41.yaml): Fedora 41
- [`fedora-42`](./fedora-42.yaml): Fedora 42
- [`fedora-43`](./fedora-43.yaml), `fedora`: ⭐Fedora 43
- [`opensuse-leap-15`](./opensuse-leap-15.yaml): openSUSE Leap 15
- [`opensuse-leap-16`](./opensuse-leap-16.yaml), `opensuse-leap`, `opensuse`: ⭐openSUSE Leap 16
- [`oraclelinux-8`](./oraclelinux-8.yaml): Oracle Linux 8
- [`oraclelinux-9`](./oraclelinux-9.yaml): Oracle Linux 9
- [`oraclelinux-10`](./oraclelinux-10.yaml), `oraclelinux`: Oracle Linux 10
- [`rocky-8`](./rocky-8.yaml): Rocky Linux 8
- [`rocky-9`](./rocky-9.yaml): Rocky Linux 9
- [`rocky-10`](./rocky-10.yaml), `rocky`: Rocky Linux 10
- [`ubuntu-20.04`](./ubuntu-20.04.yaml): Ubuntu 20.04 LTS (Focal Fossa)
- [`ubuntu-22.04`](./ubuntu-22.04.yaml): Ubuntu 22.04 LTS (Jammy Jellyfish)
- [`ubuntu-24.04`](./ubuntu-24.04.yaml), `ubuntu-lts`: Ubuntu 24.04 LTS (Noble Numbat)
- [`ubuntu-24.10`](./ubuntu-24.10.yaml): Ubuntu 24.10 (Oracular Oriole)
- [`ubuntu-25.04`](./ubuntu-25.04.yaml): Ubuntu 25.04 (Plucky Puffin)
- [`ubuntu-25.10`](./ubuntu-25.10.yaml), `ubuntu`: Ubuntu 25.10 (Questing Quokka)
  - Same as `default` but comment lines are omitted from the YAML
- [`ubuntu-26.04`](./ubuntu-26.04.yaml): Ubuntu 26.04 (Resolute Raccoon)
- [`experimental/ubuntu-next`](./experimental/ubuntu-next.yaml): Ubuntu vNext
- [`experimental/gentoo`](./experimental/gentoo.yaml): Gentoo
- [`experimental/opensuse-tumbleweed`](./experimental/opensuse-tumbleweed.yaml): openSUSE Tumbleweed
- [`experimental/debian-sid`](./experimental/debian-sid.yaml): Debian Sid
- [`experimental/fedora-rawhide`](./experimental/fedora-rawhide.yaml): Fedora Rawhide

### Non-Linux

NOTE: support for non-Linux OSes is [experimental](https://lima-vm.io/docs/releases/experimental/).

- [`macos-15`](./macos-15.yaml): [macOS](https://lima-vm.io/docs/usage/guests/macos/) 15 (Sequoia)
- [`macos-26`](./macos-26.yaml), `macos`: [macOS](https://lima-vm.io/docs/usage/guests/macos/) 26 (Tahoe)
- [`freebsd-15`](./freebsd-15.yaml), `freebsd`: [FreeBSD](https://lima-vm.io/docs/usage/guests/freebsd/) 15
- [`experimental/freebsd-current`](./experimental/freebsd-current.yaml): [FreeBSD](https://lima-vm.io/docs/usage/guests/freebsd/) CURRENT

### Alternative package managers

- [`linuxbrew`](./linuxbrew.yaml), `homebrew-linux`: [Homebrew](https://brew.sh) on Linux (Ubuntu)
- [`homebrew-macos`](./homebrew-macos.yaml): [Homebrew](https://brew.sh) on macOS

### Containers

#### Container engines

- [`apptainer`](./apptainer.yaml): [Apptainer](https://lima-vm.io/docs/examples/containers/apptainer/)
- [`apptainer-rootful`](./apptainer-rootful.yaml): [Apptainer](https://lima-vm.io/docs/examples/containers/apptainer/) (rootful)
- [`docker`](./docker.yaml): ⭐[Docker](https://lima-vm.io/docs/examples/containers/docker/)
- [`docker-rootful`](./docker-rootful.yaml): [Docker](https://lima-vm.io/docs/examples/containers/docker/) (rootful)
- [`podman`](./podman.yaml): [Podman](https://lima-vm.io/docs/examples/containers/podman/)
- [`podman-rootful`](./podman-rootful.yaml): [Podman](https://lima-vm.io/docs/examples/containers/podman/) (rootful)
- LXD is installed in the default Ubuntu template, so there is no `lxd`

#### Container image builders

- [`buildkit`](./buildkit.yaml): BuildKit

#### Container orchestration

- [`faasd`](./faasd.yaml): [Faasd](https://docs.openfaas.com/deployment/edge/)
- [`k0s`](./k0s.yaml): [k0s](https://k0sproject.io/) Zero Friction Kubernetes
- [`k3s`](./k3s.yaml): Kubernetes via k3s
- [`k8s`](./k8s.yaml): ⭐[Kubernetes](https://lima-vm.io/docs/examples/containers/kubernetes/) via kubeadm
- [`experimental/rke2`](./experimental/rke2.yaml): RKE2
- [`experimental/u7s`](./experimental/u7s.yaml): [Usernetes](https://github.com/rootless-containers/usernetes): Rootless Kubernetes

### AI agents

See <https://lima-vm.io/docs/examples/ai/>.

### Optional feature enablers

- [`experimental/vnc`](./experimental/vnc.yaml): use vnc display and xorg server
- [`experimental/alsa`](./experimental/alsa.yaml): use alsa and default audio device

### Lost+found

<details>
<summary>Details</summary>
<p>

- `centos`: Removed in Lima v0.8.0, as CentOS 8 reached [EOL](https://www.centos.org/centos-linux-eol/).
  Replaced by [`almalinux`](./almalinux.yaml), [`centos-stream`](./centos-stream.yaml), [`oraclelinux`](./oraclelinux.yaml),
  and [`rocky`](./rocky.yaml).
- `singularity`: Moved to [`apptainer-rootful`](./apptainer-rootful.yaml) in Lima v0.13.0, as Singularity was renamed to Apptainer.
- `experimental/apptainer`: Moved to [`apptainer`](./apptainer.yaml) in Lima v0.13.0.
- `experimental/{almalinux,centos-stream-9,oraclelinux,rocky}-9`: Moved to [`almalinux-9`](./almalinux-9.yaml), [`centos-stream-9`](./centos-stream-9.yaml),
  [`oraclelinux-9`](./oraclelinux-9.yaml), and [`rocky-9`](./rocky-9.yaml) in Lima v0.13.0.
- `nomad`: Removed in Lima v0.17.1, as Nomad is [no longer free software](https://github.com/hashicorp/nomad/commit/b3e30b1dfa185d9437a25830522da47b91f78816)
- `centos-stream-8`: Remove in Lima v0.23.0, as CentOS Stream 8 reached [EOL](https://blog.centos.org/2023/04/end-dates-are-coming-for-centos-stream-8-and-centos-linux-7/).
- `deprecated/centos-7`: Remove in Lima v0.23.0, as CentOS 7 reached [EOL](https://blog.centos.org/2023/04/end-dates-are-coming-for-centos-stream-8-and-centos-linux-7/).
- `experimental/vz`: Merged into the default template in Lima v1.0. See also <https://lima-vm.io/docs/config/vmtype/>.
- `experimental/armv7l`: Merged into the `default` template in Lima v1.0. Use `limactl create --arch=armv7l template:default`.
- `experimental/riscv64`: Merged into the `default` template in Lima v1.0. Use `limactl create --arch=riscv64 template:default`.
- `vmnet`: Removed in Lima v1.0. Use `limactl create --network=lima:shared template:default` instead. See also <https://lima-vm.io/docs/config/network/>.
- `experimental/net-user-v2`: Removed in Lima v1.0. Use `limactl create --network=lima:user-v2 template:default` instead. See also <https://lima-vm.io/docs/config/network/>.
- `experimental/9p`: Removed in Lima v1.0. Use `limactl create --vm-type=qemu --mount-type=9p template:default` instead. See also <https://lima-vm.io/docs/config/mount/>.
- `experimental/virtiofs-linux`: Removed in Lima v1.0. Use `limactl create --mount-type=virtiofs-linux template:default` instead. See also <https://lima-vm.io/docs/config/mount/>.

</p>
</details>

## Tier

- "Tier 1" (marked with ⭐): Good stability. Regularly tested on the CI.
- "Tier 2" (marked with ☆): Moderate stability. Regularly tested on the CI.

Other templates are tested only occasionally and manually.
