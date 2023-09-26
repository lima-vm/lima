---
title: FAQs
weight: 6
---
<!-- doctoc: https://github.com/thlorenz/doctoc -->

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->


- [Generic](#generic)
  - ["How does Lima work?"](#how-does-lima-work)
  - ["What's my login password?"](#whats-my-login-password)
  - ["Does Lima work on ARM Mac?"](#does-lima-work-on-arm-mac)
  - ["Can I run non-Ubuntu guests?"](#can-i-run-non-ubuntu-guests)
  - ["Can I run other container engines such as Docker and Podman? What about Kubernetes?"](#can-i-run-other-container-engines-such-as-docker-and-podman-what-about-kubernetes)
  - ["Can I run Lima with a remote Linux machine?"](#can-i-run-lima-with-a-remote-linux-machine)
  - ["Advantages compared to Docker for Mac?"](#advantages-compared-to-docker-for-mac)
- [Configuration](#configuration)
  - ["Is it possible to disable mounts, port forwarding, containerd, etc. ?"](#is-it-possible-to-disable-mounts-port-forwarding-containerd-etc-)
- [QEMU](#qemu)
  - ["QEMU crashes with `HV_ERROR`"](#qemu-crashes-with-hv_error)
  - ["QEMU is slow"](#qemu-is-slow)
  - [error "killed -9"](#error-killed--9)
  - ["QEMU crashes with `vmx_write_mem: mmu_gva_to_gpa XXXXXXXXXXXXXXXX failed`"](#qemu-crashes-with-vmx_write_mem-mmu_gva_to_gpa-xxxxxxxxxxxxxxxx-failed)
- [VZ](#vz)
  - ["Lima gets stuck at `Installing rosetta...`"](#lima-gets-stuck-at-installing-rosetta)
- [Networking](#networking)
  - ["Cannot access the guest IP 192.168.5.15 from the host"](#cannot-access-the-guest-ip-192168515-from-the-host)
  - ["Ping shows duplicate packets and massive response times"](#ping-shows-duplicate-packets-and-massive-response-times)
  - ["IP address is not assigined for vmnet networks"](#ip-address-is-not-assigined-for-vmnet-networks)
- [Filesystem sharing](#filesystem-sharing)
  - ["Filesystem is slow"](#filesystem-is-slow)
  - ["Filesystem is not writable"](#filesystem-is-not-writable)
- [External projects](#external-projects)
  - ["I am using Rancher Desktop. How to deal with the underlying Lima?"](#i-am-using-rancher-desktop-how-to-deal-with-the-underlying-lima)
- ["Hints for debugging other problems?"](#hints-for-debugging-other-problems)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

### Generic
#### "How does Lima work?"

- Hypervisor: [QEMU with HVF accelerator (default), or Virtualization.framework](../config/vmtype/)
- Filesystem sharing: [Reverse SSHFS (default),  or virtio-9p-pci aka virtfs, or virtiofs](../config/mount/)
- Port forwarding: `ssh -L`, automated by watching `/proc/net/tcp` and `iptables` events in the guest

#### "What's my login password?"
Password is disabled and locked by default.
You have to use `limactl shell bash` (or `lima bash`) to open a shell.

Alternatively, you may also directly ssh into the guest: `ssh -p 60022 -i ~/.lima/_config/user -o NoHostAuthenticationForLocalhost=yes 127.0.0.1`.

#### "Does Lima work on ARM Mac?"
Yes, it should work, but not regularly tested on ARM (due to lack of CI).

#### "Can I run non-Ubuntu guests?"
AlmaLinux, Alpine, Arch Linux, Debian, Fedora, openSUSE, Oracle Linux, and Rocky are also known to work.
{{% fixlinks %}}
See [`./examples/`](./examples/).
{{% /fixlinks %}}

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
{{% fixlinks %}}
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

The default Ubuntu image also contains LXD. Run `lima sudo lxc init` to set up LXD.

See also third party containerd projects based on Lima:
- [Rancher Desktop](https://rancherdesktop.io/): Kubernetes and container management to the desktop
- [Colima](https://github.com/abiosoft/colima): Docker (and Kubernetes) on macOS with minimal setup

Or third party "[containers](https://github.com/containers)" projects compatible with Lima:
- [Podman Desktop](https://podman-desktop.io/): Containers and Kubernetes for application developers

{{% /fixlinks %}}

#### "Can I run Lima with a remote Linux machine?"
Lima itself does not support connecting to a remote Linux machine, but [sshocker](https://github.com/lima-vm/sshocker),
the predecessor or Lima, provides similar features for remote Linux machines.

e.g., run `sshocker -v /Users/foo:/home/foo/mnt -p 8080:80 <USER>@<HOST>` to expose `/Users/foo` to the remote machine as `/home/foo/mnt`,
and forward `localhost:8080` to the port 80 of the remote machine.

#### "Advantages compared to Docker for Mac?"
Lima is free software (Apache License 2.0), while Docker for Mac is not.

### Configuration
#### "Is it possible to disable mounts, port forwarding, containerd, etc. ?"

Yes, since Lima v0.18:

{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --plain
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
plain: true
```
{{% /tab %}}
{{< /tabpane >}}


When the "plain" mode is enabled:
- the YAML properties for mounts, port forwarding, containerd, etc. will be ignored
- guest agent will not be running
- dependency packages like sshfs will not be installed into the VM

User-specified provisioning scripts will be still executed.

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

```xml
    <key>com.apple.vm.hypervisor</key>
    <true/>
```

#### "QEMU is slow"
{{% fixlinks %}}
- Make sure that HVF is enabled with `com.apple.security.hypervisor` entitlement. See ["QEMU crashes with `HV_ERROR`"](#qemu-crashes-with-hv_error).
- Emulating non-native machines (ARM-on-Intel, Intel-on-ARM) is slow by design. See [`docs/multi-arch.md`](./docs/multi-arch.md) for a workaround.
{{% /fixlinks %}}

#### error "killed -9"
- make sure qemu is codesigned, See ["QEMU crashes with `HV_ERROR`"](#qemu-crashes-with-hv_error).
- if you are on macOS 10.15.7 or 11.0 or later make sure the entitlement `com.apple.vm.hypervisor` is **not** added. It only works on older macOS versions. You can clear the codesigning with `codesign --remove-signature /usr/local/bin/qemu-system-x86_64` and [start over](../installation/).

#### "QEMU crashes with `vmx_write_mem: mmu_gva_to_gpa XXXXXXXXXXXXXXXX failed`"
This error is known to happen when running an image of RHEL8-compatible distribution such as Rocky Linux 8.x on Intel Mac.
A workaround is to set environment variable `QEMU_SYSTEM_X86_64="qemu-system-x86_64 -cpu Haswell-v4"`.

<https://bugs.launchpad.net/qemu/+bug/1838390>

### VZ
#### "Lima gets stuck at `Installing rosetta...`"

Try `softwareupdate --install-rosetta` from a terminal.

### Networking
#### "Cannot access the guest IP 192.168.5.15 from the host"
{{% fixlinks %}}
The default guest IP 192.168.5.15 is not accessible from the host and other guests.

To add another IP address that is accessible from the host and other virtual machines, enable [`socket_vmnet`](https://github.com/lima-vm/socket_vmnet) (since Lima v0.12)
or [`vde_vmnet`](https://github.com/lima-vm/vde_vmnet) (Deprecated).

See [`./docs/network.md`](./docs/network.md).
{{% /fixlinks %}}

#### "Ping shows duplicate packets and massive response times"

Lima uses QEMU's SLIRP networking which does not support `ping` out of the box:

```console
$ ping google.com
PING google.com (172.217.165.14): 56 data bytes
64 bytes from 172.217.165.14: seq=0 ttl=42 time=2395159.646 ms
64 bytes from 172.217.165.14: seq=0 ttl=42 time=2396160.798 ms (DUP!)
```

For more details, see [Documentation/Networking](https://wiki.qemu.org/Documentation/Networking#User_Networking_.28SLIRP.29).

#### "IP address is not assigined for vmnet networks"
Try the following commands:
```bash
/usr/libexec/ApplicationFirewall/socketfilterfw --add /usr/libexec/bootpd
/usr/libexec/ApplicationFirewall/socketfilterfw --unblock /usr/libexec/bootpd
```

### Filesystem sharing
#### "Filesystem is slow"
{{% fixlinks %}}
Try virtiofs. See [`docs/mount.md`](./docs/mount.md)
{{% /fixlinks %}}

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
