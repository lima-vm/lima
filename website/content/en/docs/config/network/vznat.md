---
title: vzNAT network
weight: 33
---

| ⚡ Requirement | Lima >= 0.14, macOS >= 13.0 |
|-------------------|-----------------------------|

vzNAT assigns a "real" IP address that is reachable from the host.

Compared to [managed networks]({{< ref "/docs/config/network/managed" >}}), the "vzNAT" network has the following pros and cons:
- Pros:
  - does not need the `socket_vmnet` binary and the `sudoers` file.
  - has a significant advantage in the throughput. See the [benchmark result]({{< ref "/docs/config/port#benchmarks" >}}).
- Cons:
  - does not let guests reach each other directly, because macOS blocks traffic between vzNAT guests on the host's bridge. See [Guest-to-guest traffic](#guest-to-guest-traffic).

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

## Guest-to-guest traffic

macOS blocks direct traffic between vzNAT guests, but they can still reach each other with a route through the host. Run this in each guest, with the guest's own address in place of `192.168.64.15`. The command assumes that vzNAT is the first entry in `networks`, so its interface in the guest is `lima0`. If `ip route show dev lima0` shows another subnet or gateway, use those.

```bash
sudo ip route replace 192.168.64.0/24 via 192.168.64.1 dev lima0 metric 100 src 192.168.64.15
```

A guest restart removes the route. So does anything that reconfigures `lima0`, such as `netplan apply` on Ubuntu or taking the link down and up. After any of these, run the command again.

A guest that adds the route can send packets to any other vzNAT guest, even one without the route. So the bridge does not protect vzNAT guests from each other.

Compared to vzNAT with the route, [`user-v2`]({{< ref "/docs/config/network/user-v2" >}}) has the following pros and cons:
- Pros:
  - connects guests without a route.
  - works with QEMU too.
- Cons:
  - does not let the host reach a user-v2 guest IP directly.
  - is slower for traffic between guests, which passes through the `limactl usernet` process on the host.
  - makes that process use a lot of host CPU under heavy traffic.
