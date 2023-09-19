---
title: Usage
weight: 2
---

## Start a linux instance

```console
limactl start [--name=NAME] [--tty=false] <template://TEMPLATE>
```

```console
$ limactl start
? Creating an instance "default"  [Use arrows to move, type to filter]
> Proceed with the current configuration
  Open an editor to review or modify the current configuration
  Choose another template (docker, podman, archlinux, fedora, ...)
  Exit
...
INFO[0029] READY. Run `lima` to open the shell.
```

Choose `Proceed with the current configuration`, and wait until "READY" to be printed on the host terminal.

For automation,  `--tty=false` flag can be used for disabling the interactive user interface.

### Advanced usage
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
$ limactl start --name=default /usr/local/share/lima/templates/fedora.yaml
```

To create an instance "default" from a remote URL (use carefully, with a trustable source):
```console
$ limactl start --name=default https://raw.githubusercontent.com/lima-vm/lima/master/examples/alpine.yaml
```

#### limactl shell
`limactl shell <INSTANCE> <COMMAND>`: launch `<COMMAND>` on Linux.

For the "default" instance, this command can be shortened as `lima <COMMAND>`.
The `lima` command also accepts the instance name as the environment variable `$LIMA_INSTANCE`.

SSH can be used too:
```console
$ limactl ls --format='{{.SSHConfigFile}}' default
/Users/example/.lima/default/ssh.config

$ ssh -F /Users/example/.lima/default/ssh.config lima-default
```

#### limactl completion
- To enable bash completion, add `source <(limactl completion bash)` to `~/.bash_profile`.

- To enable zsh completion, see `limactl completion zsh --help`

## Configuration
{{% fixlinks %}}

See [`./examples/default.yaml`](./examples/default.yaml).

The current default spec:
- OS: Ubuntu 23.04 (Lunar Lobster)
- CPU: 4 cores
- Memory: 4 GiB
- Disk: 100 GiB
- Mounts: `~` (read-only), `/tmp/lima` (writable)
- SSH: 127.0.0.1:60022

## How it works

- Hypervisor: [QEMU with HVF accelerator (default), or Virtualization.framework](./docs/vmtype.md)
- Filesystem sharing: [Reverse SSHFS (default),  or virtio-9p-pci aka virtfs, or virtiofs](./docs/mount.md)
- Port forwarding: `ssh -L`, automated by watching `/proc/net/tcp` and `iptables` events in the guest

{{% /fixlinks %}}
