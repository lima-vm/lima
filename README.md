Translations: [日本語(Japanese)](README.ja.md) [简体中文（Simplified Chinese）](README.zh.md)

[[📖**Getting started**]](#getting-started)
[[❓**FAQs & Troubleshooting]**](#faqs--troubleshooting)

![Lima logo](./docs/images/lima-logo-01.svg)

# Lima: Linux virtual machines (on macOS, in most cases)

Lima launches Linux virtual machines with automatic file sharing and port forwarding (similar to WSL2), and [containerd](https://containerd.io).

Lima can be considered as a some sort of unofficial "containerd for Mac".

Lima is expected to be used on macOS hosts, but can be used on Linux hosts as well.

✅ Automatic file sharing

✅ Automatic port forwarding

✅ Built-in support for [containerd](https://containerd.io) ([Other container engines can be used too](./examples))

✅ Intel on Intel

✅ [ARM on Intel](./docs/multi-arch.md)

✅ ARM on ARM

✅ [Intel on ARM](./docs/multi-arch.md)

✅ Various guest Linux distributions: [AlmaLinux](./examples/almalinux.yaml), [Alpine](./examples/alpine.yaml), [Arch Linux](./examples/archlinux.yaml), [Debian](./examples/debian.yaml), [Fedora](./examples/fedora.yaml), [openSUSE](./examples/opensuse.yaml), [Oracle Linux](./examples/oraclelinux.yaml), [Rocky](./examples/rocky.yaml), [Ubuntu](./examples/ubuntu.yaml) (default), ...

Related project: [sshocker (ssh with file sharing and port forwarding)](https://github.com/lima-vm/sshocker)

This project is unrelated to [The Lima driver project (driver for ARM Mali GPUs)](https://gitlab.freedesktop.org/lima).

The [talks](docs/talks.md) page contains links to slides and video from conference presentations about Lima.

## Motivation

The goal of Lima is to promote [containerd](https://containerd.io) including [nerdctl (contaiNERD ctl)](https://github.com/containerd/nerdctl)
to Mac users, but Lima can be used for non-container applications as well.

## Community
### Adopters

Container environments:
- [Rancher Desktop](https://rancherdesktop.io/): Kubernetes and container management to the desktop
- [Colima](https://github.com/abiosoft/colima): Docker (and Kubernetes) on macOS with minimal setup
- [Finch](https://github.com/runfinch/finch): Finch is a command line client for local container development

GUI:
- [Lima xbar plugin](https://github.com/unixorn/lima-xbar-plugin): [xbar](https://xbarapp.com/) plugin to start/stop VMs from the menu bar and see their running status.
- [lima-gui](https://github.com/afbjorklund/lima-gui): Qt GUI for Lima

### Communication channels
- [GitHub Discussions](https://github.com/lima-vm/lima/discussions)
- `#lima` channel in the CNCF Slack
  - New account: https://slack.cncf.io/
  - Login: https://cloud-native.slack.com/

### Code of Conduct
Lima follows the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md).

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

> You don't need to run "lima nerdctl" everytime, instead you can use special shortcut called "nerdctl.lima" to do the same thing. By default, it'll be installed along with the lima, so, you don't need to do anything extra. There will be a symlink called nerdctl pointing to nerdctl.lima. This is only created when there is no nerdctl entry in the directory already though. It worths to mention that this is created only via make install. Not included in Homebrew/MacPorts/nix packages.

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

Install QEMU 7.0 or later.

#### Install Lima

- Download the binary archive of Lima from https://github.com/lima-vm/lima/releases ,
and extract it under `/usr/local` (or somewhere else). For instance:

```bash
brew install jq
VERSION=$(curl -fsSL https://api.github.com/repos/lima-vm/lima/releases/latest | jq -r .tag_name)
curl -fsSL "https://github.com/lima-vm/lima/releases/download/${VERSION}/lima-${VERSION:1}-$(uname -s)-$(uname -m).tar.gz" | tar Cxzvm /usr/local
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

### Command reference

#### `limactl start`
`limactl start [--name=NAME] [--tty=false] <template://TEMPLATE>`: start the Linux instance

```console
$ limactl start
? Creating an instance "default"  [Use arrows to move, type to filter]
> Proceed with the current configuration
  Open an editor to review or modify the current configuration
  Choose another example (docker, podman, archlinux, fedora, ...)
  Exit
...
INFO[0029] READY. Run `lima` to open the shell.
```

Choose `Proceed with the current configuration`, and wait until "READY" to be printed on the host terminal.
For automation,  `--tty=false` flag can be used for disabling the interactive user interface.

##### Advanced usage
To create an instance "default" from a template "docker":
```console
$ limactl start --name=default template://docker
```

> NOTE: `limactl start template://TEMPLATE` requires Lima v0.9.0 or later.
> Older releases require `limactl start /usr/local/share/doc/lima/examples/TEMPLATE.yaml` instead.

To create an instance "default" with modified parameters:
```console
$ limactl start --set='.cpus = 2 | .memory = "2GiB"'
```

To see the template list:
```console
$ limactl start --list-templates
```

To create an instance "default" from a local file:
```console
$ limactl start --name=default /usr/local/share/lima/examples/fedora.yaml
```

To create an instance "default" from a remote URL (use carefully, with a trustable source):
```console
$ limactl start --name=default https://raw.githubusercontent.com/lima-vm/lima/master/examples/alpine.yaml
```

#### `limactl shell`
`limactl shell <INSTANCE> <COMMAND>`: launch `<COMMAND>` on Linux.

For the "default" instance, this command can be shortened as `lima <COMMAND>`.
The `lima` command also accepts the instance name as the environment variable `$LIMA_INSTANCE`.

#### `limactl copy`
`limactl copy <SOURCE> ... <TARGET>`: copy files between instances, or between instances and the host

Use `<INSTANCE>:<FILENAME>` to specify a source or target inside an instance.

#### `limactl list`
`limactl list [--json]`: show the instances

#### `limactl stop`
`limactl stop [--force] <INSTANCE>`: stop the instance

#### `limactl delete`
`limactl delete [--force] <INSTANCE>`: delete the instance

#### `limactl factory-reset`
`limactl factory-reset <INSTANCE>`: factory reset the instance

#### `limactl edit`
`limactl edit <INSTANCE>`: edit the instance

#### `limactl disk`

`limactl disk create <DISK> --size <SIZE> [--format qcow2]`: create a new external disk to attach to an instance

`limactl disk delete <DISK>`: delete an existing disk

`limactl disk list`: list all existing disks

#### `limactl show-ssh`
- `limactl show-ssh --format=cmd <INSTANCE>` (default): Full `ssh` command line
- `limactl show-ssh --format=args <INSTANCE>`: Similar to the `cmd` format but omits `ssh` and the destination address
- `limactl show-ssh --format=options <INSTANCE>`: ssh option key value pairs
- `limactl show-ssh --format=config <INSTANCE>`: `~/.ssh/config` format

The config file is also automatically created inside the instance directory:
```console
$ limactl ls --format='{{.SSHConfigFile}}' default
/Users/example/.lima/default/ssh.config

$ ssh -F /Users/example/.lima/default/ssh.config lima-default
```

#### `limactl snapshot`
`limactl snapshot <COMMAND> <INSTANCE>`: manage instance snapshots

Commands:
`limactl snapshot create --tag TAG INSTANCE` : create (save) a snapshot
`limactl snapshot apply --tag TAG INSTANCE` : apply (load) a snapshot
`limactl snapshot delete --tag TAG INSTANCE` : delete (del) a snapshot
`limactl snapshot list INSTANCE` : list existing snapshots in instance

#### `limactl completion`
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

See [`./examples/default.yaml`](./examples/default.yaml).

The current default spec:
- OS: Ubuntu 22.10 (Kinetic Kudu)
- CPU: 4 cores
- Memory: 4 GiB
- Disk: 100 GiB
- Mounts: `~` (read-only), `/tmp/lima` (writable)
- SSH: 127.0.0.1:60022

## How it works

- Hypervisor: [QEMU with HVF accelerator (default), or Virtualization.framework](./docs/vmtype.md)
- Filesystem sharing: [Reverse SSHFS (default),  or virtio-9p-pci aka virtfs, or virtiofs](./docs/mount.md)
- Port forwarding: `ssh -L`, automated by watching `/proc/net/tcp` and `iptables` events in the guest

## Developer guide

### Contributing to Lima
- Please certify your [Developer Certificate of Origin (DCO)](https://developercertificate.org/),
  by signing off your commit with `git commit -s` and with your real name.

- Please squash commits.

### Help wanted
:pray:
- Documents
- CLI user experience
- Performance optimization
- Windows hosts
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
- [Networking](#networking)
  - ["Cannot access the guest IP 192.168.5.15 from the host"](#cannot-access-the-guest-ip-192168515-from-the-host)
  - ["Ping shows duplicate packets and massive response times"](#ping-shows-duplicate-packets-and-massive-response-times)
- [Filesystem sharing](#filesystem-sharing)
  - ["Filesystem is slow"](#filesystem-is-slow)
  - ["Filesystem is not writable"](#filesystem-is-not-writable)
- [External projects](#external-projects)
  - ["I am using Rancher Desktop. How to deal with the underlying Lima?"](#i-am-using-rancher-desktop-how-to-deal-with-the-underlying-lima)
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
AlmaLinux, Alpine, Arch Linux, Debian, Fedora, openSUSE, Oracle Linux, and Rocky are also known to work.
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
- [`./examples/apptainer.yaml`](./examples/apptainer.yaml): Apptainer

Container image builder examples:
- [`./examples/buildkit.yaml`](./examples/buildkit.yaml): BuildKit

Container orchestrator examples:
- [`./examples/k3s.yaml`](./examples/k3s.yaml): Kubernetes (k3s)
- [`./examples/k8s.yaml`](./examples/k8s.yaml): Kubernetes (kubeadm)
- [`./examples/nomad.yaml`](./examples/nomad.yaml): Nomad

The default Ubuntu image also contains LXD. Run `lima sudo lxc init` to set up LXD.

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

### Networking
#### "Cannot access the guest IP 192.168.5.15 from the host"

The default guest IP 192.168.5.15 is not accessible from the host and other guests.

To add another IP address that is accessible from the host and other virtual machines, enable [`socket_vmnet`](https://github.com/lima-vm/socket_vmnet) (since Lima v0.12)
or [`vde_vmnet`](https://github.com/lima-vm/vde_vmnet) (Deprecated).

See [`./docs/network.md`](./docs/network.md).

#### "Ping shows duplicate packets and massive response times"

Lima uses QEMU's SLIRP networking which does not support `ping` out of the box:

```
$ ping google.com
PING google.com (172.217.165.14): 56 data bytes
64 bytes from 172.217.165.14: seq=0 ttl=42 time=2395159.646 ms
64 bytes from 172.217.165.14: seq=0 ttl=42 time=2396160.798 ms (DUP!)
```

For more details, see [Documentation/Networking](https://wiki.qemu.org/Documentation/Networking#User_Networking_.28SLIRP.29).

### Filesystem sharing
#### "Filesystem is slow"
Try virtiofs. See [`docs/mount.md`](./docs/mount.md)

#### "Filesystem is not writable"
The home directory is mounted as read-only by default.
To enable writing, specify `writable: true` in the YAML:

```yaml
mounts:
- location: "~"
  writable: true
```

Run `limactl edit <INSTANCE>` to open the YAML editor for an existing instance.

### External projects
#### "I am using Rancher Desktop. How to deal with the underlying Lima?"

Rancher Desktop includes the `rdctl` tool (installed in `~/.rd/bin/rdctl`) that provides shell access via `rdctl shell`.

It is not recommended to directly interact with the Rancher Desktop VM via `limactl`.

If you need to create an `override.yaml` file, its location should be:

* macOS: `$HOME/Library/Application Support/rancher-desktop/lima/_config/override.yaml`
* Linux: `$HOME/.local/share/rancher-desktop/lima/_config/override.yaml`

### "Hints for debugging other problems?"
- Inspect logs:
  - `limactl --debug start`
  - `$HOME/.lima/<INSTANCE>/serial.log`
  - `/var/log/cloud-init-output.log` (inside the guest)
  - `/var/log/cloud-init.log` (inside the guest)
- Make sure that you aren't mixing up tabs and spaces in the YAML.

- - -
**We are a [Cloud Native Computing Foundation](https://cncf.io/) sandbox project.**

<img src="https://www.cncf.io/wp-content/uploads/2022/07/cncf-color-bg.svg" width=300 />

The Linux Foundation® (TLF) has registered trademarks and uses trademarks. For a list of TLF trademarks, see [Trademark Usage](https://www.linuxfoundation.org/trademark-usage/).
