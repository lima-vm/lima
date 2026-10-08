---
id: lima
summary: Lima launches Linux virtual machines on a developer's own computer, with automatic file sharing and port forwarding, from ready-made templates.
category: developer-tools
tags:
  - virtual-machines
  - linux
  - containers
  - command-line
authors:
  - name: The Lima Authors
    url: https://lima-vm.io
license: Apache-2.0
languages:
  - en
limitations:
  - limactl never runs as the root user; on macOS the privileged helpers it needs are authorized through a sudoers file the user installs.
  - Lima boots operating system images published by their distributors; it downloads them and does not build them.
  - Instances are reached over SSH, using the host's own ssh, scp and rsync.
  - 'Virtual machines run on the host''s own virtualization: QEMU, Apple Virtualization.framework, krunkit, WSL2 or Hyper-V.'
  - What an instance can use (mount types, networks, displays, snapshots, host block devices) depends on the host, the guest and the VM type.
  - Snapshots are available only for instances that run on QEMU or krunkit.
  - Disks never shrink.
  - Protection guards an instance against Lima's own delete and factory reset, not against removing its files by other means.
  - Automatic startup is available on macOS and Linux hosts; startup at boot only on macOS.
  - The sudoers file for privileged networking and host block devices is generated for macOS hosts only, and installing it is left to the user.
  - Applications inside the guest, including container engines and Kubernetes, are operated with their own tools; Lima runs those tools in the instance.
references:
  - kind: code
    role: implementation
    target: cmd/limactl/main.go#newApp
  - kind: doc
    role: context
    target: README.md
  - kind: doc
    role: context
    target: website/content/en/docs/_index.md
---

# Lima

Lima launches Linux virtual machines — *instances* — on the user's own macOS,
Linux or Windows computer, similar to WSL2. An instance is created from a
template, boots a distributor's image, shares the user's home directory with the
guest and forwards the ports the guest listens on to the host's localhost, so
programs inside the guest feel local. Lima ships templates for common Linux
distributions and for container engines (containerd with nerdctl, Docker,
Podman, Kubernetes), and keeps snapshots, extra disks and networks for its
instances. People use it through the `limactl` and `lima` commands, and AI
agents can work inside an instance through its experimental MCP server.

## Intent

Give developers a disposable, local Linux machine that behaves as if it were
part of their host — originally to bring containerd and nerdctl to Mac users,
and equally for any non-container workload.
