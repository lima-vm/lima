---
title: vzNAT network
weight: 33
---

| ⚡ Requirement | Lima >= 0.14, macOS >= 13.0 |
|-------------------|-----------------------------|

vzNAT assigns a "real" IP address that is reachable from the host.

Compared to [managed networks]({{< ref "/docs/config/network/managed" >}}), the "vzNAT" network:
- does not need the `socket_vmnet` binary and the `sudoers` file.
- has a significant advantage in the throughput. See the [benchmark result]({{< ref "/docs/config/port#benchmarks" >}}).

For [VZ]({{< ref "/docs/config/vmtype#vz" >}}) instances, the "vzNAT" network can be configured as follows:
{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --vm-type=vz --network=vzNAT
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
networks:
- vzNAT: true
```
{{% /tab %}}
{{< /tabpane >}}

The range of the IP address is not specifiable.
