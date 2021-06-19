[[⬇️ **Download]**](https://github.com/AkihiroSuda/lima/releases)
[[📖**Getting started]**](#getting-started)
[[❓**FAQs & Troubleshooting]**](#faqs--troubleshooting)

# Lima: Linux virtual machines (on macOS, in most cases)

Lima launches Linux virtual machines with automatic file sharing, port forwarding, and [containerd](https://containerd.io).

Lima can be considered as a some sort of unofficial "macOS subsystem for Linux", or "containerd for Mac".

Lima is expected to be used on macOS hosts, but can be used on Linux hosts as well.
It may work on NetBSD and Windows hosts as well.

✅ Automatic file sharing

✅ Automatic port forwarding

✅ Built-in support for [containerd](https://containerd.io)

✅ Intel on Intel

✅ ARM on Intel

✅ ARM on ARM

✅ Intel on ARM

✅ Various guest Linux distributions: Ubuntu, Debian, Fedora, ...

Related project: [sshocker (ssh with file sharing and port forwarding)](https://github.com/AkihiroSuda/sshocker)

This project is unrelated to [The Lima driver project (driver for ARM Mali GPUs)](https://gitlab.freedesktop.org/lima).

## Motivation

The goal of Lima is to promote [containerd](https://containerd.io) including [nerdctl (contaiNERD ctl)](https://github.com/containerd/nerdctl)
to Mac users, but Lima can be used for non-container applications as well.

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

### Sharing files across macOS and Linux
```console
$ echo "files under /Users on macOS filesystem are readable from Linux" > some-file

$ lima cat some-file
files under /Users on macOS filesystem are readable from Linux

$ lima sh -c 'echo "/tmp/lima is writable from both macOS and Linux" > /tmp/lima/another-file'

$ cat /tmp/lima/another-file
/tmp/lima is writable from both macOS and Linux"
```

### Running containerd containers (compatible with Docker containers)
```console
$ lima nerdctl run -d --name nginx -p 127.0.0.1:8080:80 nginx:alpine
```

http://127.0.0.1:8080 is accessible from both macOS and Linux.

> **NOTE**
> Privileged ports (1-1023) cannot be forwarded

For the usage of containerd and nerdctl (contaiNERD ctl), visit https://github.com/containerd/containerd and https://github.com/containerd/nerdctl.

## Getting started
### Requirements (Intel Mac)
- QEMU v6.0.0 or later (`brew install qemu`)

### Requirements (ARM Mac)

- QEMU with `--accel=hvf` support, see https://gist.github.com/nrjdalal/e70249bb5d2e9d844cc203fd11f74c55

> **NOTE**
> Lima is not regularly tested on ARM Mac.

### Install

Download the binary archive from https://github.com/AkihiroSuda/lima/releases ,
and extract it under `/usr/local` (or somewhere else).

To install from the source, run `make && make install`.

### Usage

```console
[macOS]$ limactl start
...
INFO[0029] READY. Run `lima` to open the shell.

[macOS]$ lima uname
Linux
```

Detailed usage:

- Run `limactl start <INSTANCE>` to start the Linux instance.
  The default instance name is "default".
  Lima automatically opens an editor (`vi`) for reviewing and modifying the configuration.
  Wait until "READY" to be printed on the host terminal.

- Run `limactl shell <INSTANCE> <COMMAND>` to launch `<COMMAND>` on Linux.
  For the "default" instance, this command can be shortened as `lima <COMMAND>`.
  The `lima` command also accepts the instance name as the environment variable `$LIMA_INSTANCE`.

- Run `limactl list [--json]` to show the instances.

- Run `limactl stop [--force] <INSTANCE>` to stop the instance.

- Run `limactl delete [--force] <INSTANCE>` to delete the instance.

- To enable bash completion, add `source <(limactl completion bash)` to `~/.bash_profile`.

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
- OS: Ubuntu 21.04 (Hirsute Hippo)
- CPU (x86\_64): Haswell v4, 4 cores
- CPU (aarch64): Cortex A72, 4 cores
- Memory: 4 GiB
- Disk: 100 GiB
- Mounts: `~` (read-only), `/tmp/lima` (writable)
- SSH: 127.0.0.1:60022

## How it works

- Hypervisor: QEMU with HVF accelerator
- Filesystem sharing: [reverse sshfs](https://github.com/AkihiroSuda/sshocker/blob/v0.1.0/pkg/reversesshfs/reversesshfs.go) (planned to be replaced with 9p soon)
- Port forwarding: `ssh -L`, automated by watching `/proc/net/tcp` in the guest

## Developer guide

### Contributing to Lima
- Please certify your [Developer Certificate of Origin (DCO)](https://developercertificate.org/),
  by signing off your commit with `git commit -s` and with your real name.

- Please squash commits.

### Help wanted
:pray:
- [Test on ARM Mac](https://github.com/AkihiroSuda/lima/issues/42)
- Performance optimization
- [Homebrew](https://github.com/AkihiroSuda/lima/issues/37)
- More guest distros
- Windows hosts
- GUI with system tray icon (Qt or Electron, for portability)
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
  - ["Can I run other container engines such as Podman?"](#can-i-run-other-container-engines-such-as-podman)
  - ["Can I run Lima with a remote Linux machine?"](#can-i-run-lima-with-a-remote-linux-machine)
  - ["Advantages compared to Docker for Mac?"](#advantages-compared-to-docker-for-mac)
- [QEMU](#qemu)
  - ["QEMU crashes with `HV_ERROR`"](#qemu-crashes-with-hv_error)
  - ["QEMU is slow"](#qemu-is-slow)
  - [error "killed -9"](#error-killed--9)
- [SSH](#ssh)
  - ["Port forwarding does not work"](#port-forwarding-does-not-work)
  - [stuck on "Waiting for the essential requirement 1 of X: "ssh"](#stuck-on-waiting-for-the-essential-requirement-1-of-x-ssh)
  - [error "field SSHPubKeys must be set"](#error-field-sshpubkeys-must-be-set)
  - [error "hostkeys_foreach failed: No such file or directory"](#error-hostkeys_foreach-failed-no-such-file-or-directory)
  - [error "failed to execute script ssh: [...] Permission denied (publickey)"](#error-failed-to-execute-script-ssh--permission-denied-publickey)
- ["Hints for debugging other problems?"](#hints-for-debugging-other-problems)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->
### Generic
#### "What's my login password?"
Password is disabled and locked by default.
You have to use `limactl shell bash` (or `lima bash`) to open a shell.

Alternatively, you may also directly ssh into the guest: `ssh -p 60022 -o NoHostAuthenticationForLocalhost=yes 127.0.0.1`.

#### "Does Lima work on ARM Mac?"
Yes, it should work, but not regularly tested on ARM.

#### "Can I run non-Ubuntu guests?"
Fedora is also known to work, see [`./examples/fedora.yaml`](./examples/fedora.yaml).
This file can be loaded with `limactl start ./examples/fedora.yaml`.

An image has to satisfy the following requirements:
- systemd
- cloud-init
- The following binaries to be preinstalled:
  - `sudo`
- The following binaries to be preinstalled, or installable via the package manager:
  - `sshfs`
  - `newuidmap` and `newgidmap`
- `apt-get` or `dnf` (if you want to contribute support for another package manager, run `git grep apt-get` to find out where to modify)

#### "Can I run other container engines such as Podman?"
Yes, if you install it.

containerd can be stopped with `systemctl --user disable --now containerd`.

#### "Can I run Lima with a remote Linux machine?"
Lima itself does not support connecting to a remote Linux machine, but [sshocker](https://github.com/AkihiroSuda/sshocker),
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
- Emulating non-native machines (ARM-on-Intel, Intel-on-ARM) is slow by design.

#### error "killed -9"
- make sure qemu is codesigned, See ["QEMU crashes with `HV_ERROR`"](#qemu-crashes-with-hv_error).
- if you are on macOS 10.15.7 or 11.0 or later make sure the entitlement `com.apple.vm.hypervisor` is **not** added. It only works on older macOS versions. You can clear the codesigning with `codesign --remove-signature /usr/local/bin/qemu-system-x86_64` and [start over](#getting-started).


### SSH
#### "Port forwarding does not work"
Privileged ports (1-1023) cannot be forwarded. e.g., you have to use 8080, not 80.

#### stuck on "Waiting for the essential requirement 1 of X: "ssh"

libslirp v4.6.0 used by QEMU is known to be [broken](https://gitlab.freedesktop.org/slirp/libslirp/-/issues/48).
If you have libslirp v4.6.0 in `/usr/local/Cellar/libslirp`, you have to upgrade it to v4.6.1 or later (`brew upgrade`).

#### error "field SSHPubKeys must be set"

Make sure you have a ssh keypair in `~/.ssh`. To create:
```
ssh-keygen -q -t rsa -N '' -f ~/.ssh/id_rsa <<<n 2>&1 >/dev/null
```

#### error "hostkeys_foreach failed: No such file or directory"
Make sure you have a ssh `known_hosts` file:
```
touch ~/.ssh/known_hosts
```

#### error "failed to execute script ssh: [...] Permission denied (publickey)"
If you have a `~/.ssh/config` with a username overwrite for all hosts, exclude `127.0.0.1` from it. Example:
```
Host * !127.0.0.1
        User root
```

### "Hints for debugging other problems?"
- Inspect logs:
  - `limactl --debug start`
  - `$HOME/.lima/<INSTANCE>/serial.log`
  - `/var/log/cloud-init-output.log` (inside the guest)
  - `/var/log/cloud-init.log` (inside the guest)
- Make sure that you aren't mixing up tabs and spaces in the YAML.
- If you have passphrases for any private key under `~/.ssh`, you will need to have `ssh-agent` running.
