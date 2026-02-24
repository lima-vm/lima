---
title: Configuration guide
weight: 5
---

For all the configuration items, see <https://github.com/lima-vm/lima/blob/master/templates/default.yaml>.

The current default spec:
- OS: Ubuntu
- CPU: 4 cores
- Memory: 4 GiB
- Disk: 100 GiB
- Mounts: `~` (read-only), `/tmp/lima` (writable; removed in Lima v2.0)
- SSH: 127.0.0.1:<Random port>

## Listing resources

Lima provides `limactl` subcommands to inspect the networks and disks that
exist on the host. These are a good starting point before creating new
instances that share resources.

### Networks

```sh
limactl network list
```

Example output:

```
NAME     GATEWAY         DNS              DHCP-RANGE
shared   192.168.105.1   192.168.105.1    192.168.105.2-192.168.105.254
user-v2  192.168.106.1   192.168.106.1    192.168.106.2-192.168.106.254
foo      192.168.42.1    -                192.168.42.2-192.168.42.254
```

To use a listed network when creating an instance:

```sh
limactl create --network lima:shared template:default
limactl create --network lima:foo    --name dev template:default
```

To create a new user-defined network and share it between two VMs:

```sh
limactl network create foo --gateway 192.168.42.1/24
limactl create --network lima:foo --name vm1 template:default
limactl create --network lima:foo --name vm2 template:default
```

To remove a network:

```sh
limactl network delete foo
# Force-delete even if instances reference it:
limactl network delete --force foo
```

### Disks

Additional data disks can be created and attached to multiple instances.

```sh
limactl disk list
```

Example output:

```
NAME     SIZE    FORMAT   INSTANCE
mydata   20GiB   qcow2    -
pgdata   50GiB   qcow2    postgres
```

The `INSTANCE` column shows which instance a disk is currently locked to.
A disk that shows `-` is available to mount.

To create a new data disk:

```sh
limactl disk create mydata --size 20
```

To resize an existing disk:

```sh
limactl disk resize mydata --size 40
```

To attach the disk to an instance at creation time, add a `disk` entry in
the instance template, or pass it at the command line:

```sh
limactl create --disk mydata template:default
```

To delete a disk:

```sh
limactl disk delete mydata
# Force-delete even if an instance holds a lock on the disk:
limactl disk delete --force mydata
```

See the [Disks](./disk/) page for more information about increasing the size of
the primary VM disk.
