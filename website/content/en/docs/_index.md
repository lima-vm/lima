---
title: "Lima: Linux Machines" 
linkTitle: Documentation
menu: {main: {weight: 20}}
weight: 20
---
{{% fixlinks %}}
Lima launches Linux virtual machines with automatic file sharing and port forwarding (similar to WSL2).

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

## Motivation

The original goal of Lima was to promote [containerd](https://containerd.io) including [nerdctl (contaiNERD ctl)](https://github.com/containerd/nerdctl)
to Mac users, but Lima can be used for non-container applications as well.
Lima also supports non-macOS hosts (Linux, NetBSD, etc.).
{{% /fixlinks %}}