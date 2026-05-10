---
title: Accelerating rootless networking with bypass4netns
linkTitle: bypass4netns
weight: 2
---

[bypass4netns](https://github.com/rootless-containers/bypass4netns) is an experimental accelerator for rootless networking.

On macOS hosts, it is highly recommended to use the [vzNAT](../../../../config/network/vmnet.md#vznat) networking in conjunction
to reduce the overhead of Lima's user-mode networking:

```bash
limactl start --network vzNAT
```

To enable bypass4netns, the daemon process (`bypass4netnsd`) has to be installed in the VM as follows:
<!-- TODO: install by default -->
```bash
lima containerd-rootless-setuptool.sh install-bypass4netnsd
```

Then run a container with an annotation `nerdctl/bypass4netns=true`:

```bash
# 192.168.64.1 is the IP address of the "bridge100" interface on the macOS host
lima nerdctl run --annotation nerdctl/bypass4netns=true alpine \
  sh -euc 'apk add iperf3 && iperf3 -c 192.168.64.1'
```

Benchmark result:

| Mode                          | Throughput     |
|-------------------------------|----------------|
| Rootless without bypass4netns | 2.30 Gbits/sec |
| Rootless with bypass4netns    | 86.0 Gbits/sec |
| Rootful                       | 90.3 Gbits/sec |

<details>
<summary>Benchmarking environment</summary>
<p>

- Lima version: 2.0.0-alpha.2
  - nerdctl 2.1.6
  - containerd 2.1.4
  - bypass4netns 0.4.2
- Container: Alpine Linux 3.22.2
  - iperf 3.19.1-r0 (apk)
- Guest: Ubuntu 25.04
- Host: macOS 26.0.1
  - iperf 3.19.1 (Homebrew)
- Hardware: MacBook Pro 2024 (M4 Max, 128 GiB)

</p>
</details>
