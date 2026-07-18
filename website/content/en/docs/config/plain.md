---
title: Plain mode
weight: 60
---

Plain mode makes the VM instance as close as possible to a physical host by
disabling Lima's convenience features such as filesystem mounts, dynamic port
forwarding, and the built-in containerd.

It is useful when you want to provision the guest with plain old `ssh` and `rsync`,
without relying on Lima-specific integrations. See the
[GitHub Actions example](../examples/gha.md#plain-mode) for a typical use case.

## Enabling plain mode

{{< tabpane text=true >}}
{{% tab header="YAML" %}}
```yaml
plain: true
```
{{% /tab %}}
{{% tab header="CLI (start)" %}}
```bash
limactl start --plain
```
{{% /tab %}}
{{% tab header="CLI (edit)" %}}
```bash
limactl edit <instance> --plain
```
{{% /tab %}}
{{< /tabpane >}}

Plain mode is disabled (`false`) by default.

## What is disabled

When plain mode is enabled, the following features are turned off, and the
corresponding YAML properties are ignored:

- **Filesystem mounts** (`mounts`)
- **Dynamic port forwarding** (`portForwards` rules that are not `static: true`)
- **Built-in containerd** (`containerd.system` and `containerd.user`)
- **The guest agent daemon**
- **Rosetta** (`rosetta`)
- **SSH agent forwarding** (`ssh.forwardAgent`)
- **Host clock synchronization**

Dependency packages such as `sshfs` are not installed into the VM either.

## What still works

- **Static port forwarding**: `portForwards` rules with `static: true` are still
  established.
- **Provisioning scripts**: user, system, and data provisioning scripts are still
  executed.
- **Base guest setup**: the base user and SSH keys are still configured.

## Accessing the instance

`limactl shell <instance>` and `ssh` both work, as in a non-plain instance.
In plain mode, plain `ssh` is often preferred to keep the guest free of
Lima-specific conventions. See [SSH](../usage/ssh.md) for details.

## See also

Plain mode applies to all guest operating systems. For OS-specific differences, see
the [Guest OS](../usage/guests/_index.md) pages:

- [Linux](../usage/guests/linux.md#plain-mode)
- [FreeBSD](../usage/guests/freebsd.md#plain-mode)
- [macOS](../usage/guests/macos.md#plain-mode)

For the underlying mechanics (boot scripts, `LIMA_CIDATA_PLAIN`, guest agent
injection), see the [internals reference](../dev/internals.md).
