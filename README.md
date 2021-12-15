[[📖**Getting started]**](#getting-started)
[[❓**FAQs & Troubleshooting]**](#faqs--troubleshooting)

# Lima: Linux virtual machines (on macOS, in most cases)

Lima launches Linux virtual machines with automatic file sharing, port forwarding, and [containerd](https://containerd.io).

Lima can be considered as a some sort of unofficial "macOS subsystem for Linux", or "containerd for Mac".

Lima is expected to be used on macOS hosts, but can be used on Linux hosts as well.

✅ Automatic file sharing

✅ Automatic port forwarding

✅ Built-in support for [containerd](https://containerd.io) ([Other container engines can be used too](./examples))

✅ Intel on Intel

✅ [ARM on Intel](./docs/multi-arch.md)

✅ ARM on ARM

✅ [Intel on ARM](./docs/multi-arch.md)

✅ Various guest Linux distributions: [Alpine](./examples/alpine.yaml), [Arch Linux](./examples/archlinux.yaml), [Debian](./examples/debian.yaml), [Fedora](./examples/fedora.yaml), [openSUSE](./examples/opensuse.yaml), [Rocky](./examples/rocky.yaml), [Ubuntu](./examples/ubuntu.yaml) (default), ...

Related project: [sshocker (ssh with file sharing and port forwarding)](https://github.com/lima-vm/sshocker)

This project is unrelated to [The Lima driver project (driver for ARM Mali GPUs)](https://gitlab.freedesktop.org/lima).

## Motivation

The goal of Lima is to promote [containerd](https://containerd.io) including [nerdctl (contaiNERD ctl)](https://github.com/containerd/nerdctl)
to Mac users, but Lima can be used for non-container applications as well.

## Adopters

Container environments:
- [Rancher Desktop](https://rancherdesktop.io/): Kubernetes and container management to the desktop
- [Colima](https://github.com/abiosoft/colima): Docker (and Kubernetes) on macOS with minimal setup

GUI:
- [Lima xbar plugin](https://github.com/unixorn/lima-xbar-plugin): [xbar](https://xbarapp.com/) plugin to start/stop VMs from the menu bar and see their running status.
- [lima-gui](https://github.com/afbjorklund/lima-gui): Qt GUI for Lima

## Examples

### uname
```console
$ uname -a
Darwin macbook.local 20.4.0 Darwin Kernel Version 20.4.0: Thu Apr 22 21:46:47 PDT 2021; root:xnu-7195.101.2~1/RELEASE_X86_64 x86_64

$ lima uname -a
Linux lima-default 5.11.0-16-generic #17-Ubuntu SMP Wed Apr 14 20:12:43 UTC 2021 x86_64 x86_64 x86_64 GNU/Linux

$ LIMA_INSTANCE=arm lima uname -a
Linux lima-arm 5.11.0-16-generic #17-Ubuntu SMP Wed Apr 14 20:10:16 UTC 2021 aarch64 aarch64 aarch64 GNU/Linux
```

See [`./docs/multi-arch.md`](./docs/multi-arch.md) for Intel-on-ARM and ARM-on-Intel .

### Sharing files across macOS and Linux
```console
$ echo "files under /Users on macOS filesystem are readable from Linux" > some-file

$ lima cat some-file
files under /Users on macOS filesystem are readable from Linux

$ lima sh -c 'echo "/tmp/lima is writable from both macOS and Linux" > /tmp/lima/another-file'

$ cat /tmp/lima/another-file
/tmp/lima is writable from both macOS and Linux
```

### Running containerd containers (compatible with Docker containers)
```console
$ lima nerdctl run -d --name nginx -p 127.0.0.1:8080:80 nginx:alpine
```

http://127.0.0.1:8080 is accessible from both macOS and Linux.

For the usage of containerd and nerdctl (contaiNERD ctl), visit https://github.com/containerd/containerd and https://github.com/containerd/nerdctl.

## Getting started
### Installation

[Homebrew package](https://github.com/Homebrew/homebrew-core/blob/master/Formula/lima.rb) is available.

```console
brew install lima
```

<details>
<summary>Manual installation steps</summary>
<p>

#### Install QEMU

Install recent version of QEMU. v6.2.0 or later is recommended.

#### Install Lima

- Download the binary archive of Lima from https://github.com/lima-vm/lima/releases ,
and extract it under `/usr/local` (or somewhere else). For instance:

```bash
brew install jq
VERSION=$(curl -fsSL https://api.github.com/repos/lima-vm/lima/releases/latest | jq -r .tag_name)
curl -fsSL https://github.com/lima-vm/lima/releases/download/${VERSION}/lima-${VERSION:1}-$(uname -s)-$(uname -m).tar.gz | tar Cxzvm /usr/local
```

- To install Lima from the source, run `make && make install`.

> **NOTE**
> Lima is not regularly tested on ARM Mac (due to lack of CI).

</p>
</details>

### Usage

```console
[macOS]$ limactl start
...
INFO[0029] READY. Run `lima` to open the shell.

[macOS]$ lima uname
Linux
```

Detailed usage:

- Run `limactl start <INSTANCE> [--tty=false]` to start the Linux instance.
  The default instance name is "default".
  Lima automatically opens an editor (`vi`) for reviewing and modifying the configuration.
  Wait until "READY" to be printed on the host terminal.
  `--tty=false` disables the interactive prompt to open an editor.

- Run `limactl shell <INSTANCE> <COMMAND>` to launch `<COMMAND>` on Linux.
  For the "default" instance, this command can be shortened as `lima <COMMAND>`.
  The `lima` command also accepts the instance name as the environment variable `$LIMA_INSTANCE`.

- Run `limactl copy <SOURCE> ... <TARGET>` to copy files between instances, or between instances and the host. Use `<INSTANCE>:<FILENAME>` to specify a source or target inside an instance.

- Run `limactl list [--json]` to show the instances.

- Run `limactl stop [--force] <INSTANCE>` to stop the instance.

- Run `limactl delete [--force] <INSTANCE>` to delete the instance.

- To enable bash completion, add `source <(limactl completion bash)` to `~/.bash_profile`.

- To enable zsh completion, see `limactl completion zsh --help`

### :warning: CAUTION: make sure to back up your data
Lima may have bugs that result in loss of data.

**Make sure to back up your data before running Lima.**

Especially, the following data might be easily lost:
- Data in the shared writable directories (`/tmp/lima` by default),
  probably after hibernation of the host machine (e.g., after closing and reopening the laptop lid)
- Data in the VM image, mostly when upgrading the version of lima

### Configuration

See [`./pkg/limayaml/default.yaml`](./pkg/limayaml/default.yaml).

The current default spec:
- OS: Ubuntu 21.10 (Impish Indri)
- CPU: 4 cores
- Memory: 4 GiB
- Disk: 100 GiB
- Mounts: `~` (read-only), `/tmp/lima` (writable)
- SSH: 127.0.0.1:60022

## How it works

- Hypervisor: QEMU with HVF accelerator
- Filesystem sharing: [reverse sshfs](https://github.com/lima-vm/sshocker/blob/v0.2.0/pkg/reversesshfs/reversesshfs.go) (likely to be replaced with 9p or Samba in future)
- Port forwarding: `ssh -L`, automated by watching `/proc/net/tcp` and `iptables` events in the guest

## Developer guide

### Contributing to Lima
- Please certify your [Developer Certificate of Origin (DCO)](https://developercertificate.org/),
  by signing off your commit with `git commit -s` and with your real name.

- Please squash commits.

### Help wanted
:pray:
- Performance optimization
- More guest distros
- Windows hosts
- [VirtFS to replace the current reverse sshfs (work has to be done on QEMU repo)](https://github.com/NixOS/nixpkgs/pull/122420)
- [vsock](https://github.com/apple/darwin-xnu/blob/xnu-7195.81.3/bsd/man/man4/vsock.4) to replace SSH (work has to be done on QEMU repo)

## FAQs & Troubleshooting
<!-- doctoc: https://github.com/thlorenz/doctoc -->

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
### Generic

- [Generic](#generic)
  - ["What's my login password?"](#whats-my-login-password)
  - ["Does Lima work on ARM Mac?"](#does-lima-work-on-arm-mac)
  - ["Can I run non-Ubuntu guests?"](#can-i-run-non-ubuntu-guests)
  - ["Can I run other container engines such as Docker and Podman? What about Kubernetes?"](#can-i-run-other-container-engines-such-as-docker-and-podman-what-about-kubernetes)
  - ["Can I run Lima with a remote Linux machine?"](#can-i-run-lima-with-a-remote-linux-machine)
  - ["Advantages compared to Docker for Mac?"](#advantages-compared-to-docker-for-mac)
- [QEMU](#qemu)
  - ["QEMU crashes with `HV_ERROR`"](#qemu-crashes-with-hv_error)
  - ["QEMU is slow"](#qemu-is-slow)
  - [error "killed -9"](#error-killed--9)
  - ["QEMU crashes with `vmx_write_mem: mmu_gva_to_gpa XXXXXXXXXXXXXXXX failed`"](#qemu-crashes-with-vmx_write_mem-mmu_gva_to_gpa-xxxxxxxxxxxxxxxx-failed)
- [SSH](#ssh)
  - ["Port forwarding does not work"](#port-forwarding-does-not-work)
  - [stuck on "Waiting for the essential requirement 1 of X: "ssh"](#stuck-on-waiting-for-the-essential-requirement-1-of-x-ssh)
  - ["permission denied" for `limactl cp` command](#permission-denied-for-limactl-cp-command)
- [Networking](#networking)
  - ["Cannot access the guest IP 192.168.5.15 from the host"](#cannot-access-the-guest-ip-192168515-from-the-host)
- ["Hints for debugging other problems?"](#hints-for-debugging-other-problems)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->
### Generic
#### "What's my login password?"
Password is disabled and locked by default.
You have to use `limactl shell bash` (or `lima bash`) to open a shell.

Alternatively, you may also directly ssh into the guest: `ssh -p 60022 -i ~/.lima/_config/user -o NoHostAuthenticationForLocalhost=yes 127.0.0.1`.

#### "Does Lima work on ARM Mac?"
Yes, it should work, but not regularly tested on ARM (due to lack of CI).

#### "Can I run non-Ubuntu guests?"
Alpine, Arch Linux, Debian, Fedora, openSUSE, and Rocky are also known to work.
See [`./examples/`](./examples/).

An image has to satisfy the following requirements:
- systemd or OpenRC
- cloud-init
- The following binaries to be preinstalled:
  - `sudo`
- The following binaries to be preinstalled, or installable via the package manager:
  - `sshfs`
  - `newuidmap` and `newgidmap`
- `apt-get`, `dnf`, `apk`, `pacman`, or `zypper` (if you want to contribute support for another package manager, run `git grep apt-get` to find out where to modify)

#### "Can I run other container engines such as Docker and Podman? What about Kubernetes?"
Yes, any container engine should work with Lima.

Container runtime examples:
- [`./examples/docker.yaml`](./examples/docker.yaml): Docker
- [`./examples/podman.yaml`](./examples/podman.yaml): Podman
- [`./examples/singularity.yaml`](./examples/singularity.yaml): Singularity

Container orchestrator examples:
- [`./examples/k3s.yaml`](./examples/k3s.yaml): Kubernetes (k3s)
- [`./examples/k8s.yaml`](./examples/k8s.yaml): Kubernetes (kubeadm)
- [`./examples/nomad.yaml`](./examples/nomad.yaml): Nomad

The default Ubuntu image also contains LXD. Run`lima sudo lxc init` to set up LXD.

See also third party containerd projects based on Lima:
- [Rancher Desktop](https://rancherdesktop.io/): Kubernetes and container management to the desktop
- [Colima](https://github.com/abiosoft/colima): Docker (and Kubernetes) on macOS with minimal setup

#### "Can I run Lima with a remote Linux machine?"
Lima itself does not support connecting to a remote Linux machine, but [sshocker](https://github.com/lima-vm/sshocker),
the predecessor or Lima, provides similar features for remote Linux machines.

e.g., run `sshocker -v /Users/foo:/home/foo/mnt -p 8080:80 <USER>@<HOST>` to expose `/Users/foo` to the remote machine as `/home/foo/mnt`,
and forward `localhost:8080` to the port 80 of the remote machine.

#### "Advantages compared to Docker for Mac?"
Lima is free software (Apache License 2.0), while Docker for Mac is not.
Their [EULA](https://www.docker.com/legal/docker-software-end-user-license-agreement) even prohibits disclosure of benchmarking result.

On the other hand, [Moby](https://github.com/moby/moby), aka Docker for Linux, is free software, but Moby/Docker lacks several novel features of containerd, such as:
- [On-demand image pulling (aka lazy-pulling, eStargz)](https://github.com/containerd/nerdctl/blob/master/docs/stargz.md)
- [Running an encrypted container](https://github.com/containerd/nerdctl/blob/master/docs/ocicrypt.md)
- Importing and exporting [local OCI archives](https://github.com/opencontainers/image-spec/blob/master/image-layout.md)

### QEMU
#### "QEMU crashes with `HV_ERROR`"
If you have installed QEMU v6.0.0 or later on macOS 11 via homebrew, your QEMU binary should have been already automatically signed to enable HVF acceleration.

However, if you see `HV_ERROR`, you might need to sign the binary manually.

```bash
cat >entitlements.xml <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>com.apple.security.hypervisor</key>
    <true/>
</dict>
</plist>
EOF

codesign -s - --entitlements entitlements.xml --force /usr/local/bin/qemu-system-x86_64
```

Note: **Only** on macOS versions **before** 10.15.7 you might need to add this entitlement in addition:

```
    <key>com.apple.vm.hypervisor</key>
    <true/>
```

#### "QEMU is slow"
- Make sure that HVF is enabled with `com.apple.security.hypervisor` entitlement. See ["QEMU crashes with `HV_ERROR`"](#qemu-crashes-with-hv_error).
- Emulating non-native machines (ARM-on-Intel, Intel-on-ARM) is slow by design. See [`docs/multi-arch.md`](./docs/multi-arch.md) for a workaround.

#### error "killed -9"
- make sure qemu is codesigned, See ["QEMU crashes with `HV_ERROR`"](#qemu-crashes-with-hv_error).
- if you are on macOS 10.15.7 or 11.0 or later make sure the entitlement `com.apple.vm.hypervisor` is **not** added. It only works on older macOS versions. You can clear the codesigning with `codesign --remove-signature /usr/local/bin/qemu-system-x86_64` and [start over](#getting-started).

#### "QEMU crashes with `vmx_write_mem: mmu_gva_to_gpa XXXXXXXXXXXXXXXX failed`"
This error is known to happen when running an image of RHEL8-compatible distribution such as Rocky Linux 8.x on Intel Mac.
A workaround is to set environment variable `QEMU_SYSTEM_X86_64="qemu-system-x86_64 -cpu Haswell-v4"`.

https://bugs.launchpad.net/qemu/+bug/1838390

### SSH
#### "Port forwarding does not work"
Prior to Lima v0.7.0, Lima did not support forwarding privileged ports (1-1023). e.g., you had to use 8080, not 80.

Lima v0.7.0 and later supports forwarding privileged ports on macOS hosts.

On Linux hosts, you might have to set sysctl value `net.ipv4.ip_unprivileged_port_start=0`.

#### stuck on "Waiting for the essential requirement 1 of X: "ssh"

libslirp v4.6.0 used by QEMU is known to be [broken](https://gitlab.freedesktop.org/slirp/libslirp/-/issues/48).
If you have libslirp v4.6.0 in `/usr/local/Cellar/libslirp`, you have to upgrade it to v4.6.1 or later (`brew upgrade`).

#### "permission denied" for `limactl cp` command

The `copy` command only works for instances that have been created by lima 0.5.0 or later. You can manually install the required identity on older instances with (replace `INSTANCE` with actual instance name):

```console
< ~/.lima/_config/user.pub limactl shell INSTANCE sh -c 'tee -a ~/.ssh/authorized_keys'
```

### Networking
#### "Cannot access the guest IP 192.168.5.15 from the host"

The default guest IP 192.168.5.15 is not accessible from the host and other guests.

To add another IP address that is accessible from the host and other virtual machines, enable [`vde_vmnet`](https://github.com/lima-vm/vde_vmnet).

See [`./docs/network.md`](./docs/network.md).

### "Hints for debugging other problems?"
- Inspect logs:
  - `limactl --debug start`
  - `$HOME/.lima/<INSTANCE>/serial.log`
  - `/var/log/cloud-init-output.log` (inside the guest)
  - `/var/log/cloud-init.log` (inside the guest)
- Make sure that you aren't mixing up tabs and spaces in the YAML.
