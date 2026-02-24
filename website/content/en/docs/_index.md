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

✅ Built-in support for [containerd](https://containerd.io) ([Other container engines can be used too]({{< ref "/docs/examples/containers" >}}))

✅ Intel on Intel

✅ [ARM on Intel]({{< ref "/docs/config/multi-arch" >}})

✅ ARM on ARM

✅ [Intel on ARM]({{< ref "/docs/config/multi-arch" >}})

✅ Various guest Linux distributions: [AlmaLinux](./templates/almalinux.yaml), [Alpine](./templates/alpine.yaml), [Arch Linux](./templates/archlinux.yaml), [Debian](./templates/debian.yaml), [Fedora](./templates/fedora.yaml), [openSUSE](./templates/opensuse.yaml), [Oracle Linux](./templates/oraclelinux.yaml), [Rocky](./templates/rocky.yaml), [Ubuntu](./templates/ubuntu.yaml) (default), ...

✅ Experimental support for non-Linux guests: [macOS]({{< ref "/docs/usage/guests/macos" >}}), [FreeBSD]({{< ref "/docs/usage/guests/freebsd" >}})

Related project: [sshocker (ssh with file sharing and port forwarding)](https://github.com/lima-vm/sshocker)

This project is unrelated to [The Lima driver project (driver for ARM Mali GPUs)](https://gitlab.freedesktop.org/lima).

## Project history

Lima began in May 2021, aiming at promoting [containerd](https://containerd.io) including [nerdctl (contaiNERD ctl)](https://github.com/containerd/nerdctl)
to Mac users.
Later the project scope was expanded to support other containers
and non-container applications as well.
Lima also supports non-macOS hosts (Linux, NetBSD, etc.).

Lima joined the [Cloud Native Computing Foundation (CNCF)](https://cncf.io)
in September 2022 as a Sandbox project.
The project was promoted to the Incubating level in October 2025.

{{% /fixlinks %}}
