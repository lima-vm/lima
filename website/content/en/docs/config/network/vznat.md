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

macOS blocks direct traffic between vzNAT guests, but the host still forwards IP traffic between them. With `guestToGuest` set (available since Lima v2.3.1), Lima adds a route in the guest that sends traffic for the vzNAT subnet through the host. The guest's network service manages the route, so the route comes back whenever the guest reconfigures its network or restarts.

Set the option in every vzNAT guest that talks to another. A guest without the route sends its replies directly, and macOS blocks them.
{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --vm-type=vz --set '.networks = [{"vzNAT": true, "guestToGuest": true}]'
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
networks:
- vzNAT: true
  guestToGuest: true
```
{{% /tab %}}
{{< /tabpane >}}

The option needs a Linux guest whose network service is systemd-networkd, NetworkManager, or Alpine's dhcpcd. openSUSE Leap 15.6 uses wicked, so the option does not work there.

A guest with the route can send packets to any other vzNAT guest, even one without the option. So the bridge does not protect vzNAT guests from each other.

Compared to vzNAT with `guestToGuest`, [`user-v2`]({{< ref "/docs/config/network/user-v2" >}}) has the following pros and cons:
- Pros:
  - connects guests without a route, so it also works where `guestToGuest` does not.
  - works with QEMU too.
- Cons:
  - does not let the host reach a user-v2 guest IP directly.
  - is slower for traffic between guests, which passes through the `limactl usernet` process on the host.
  - makes that process use a lot of host CPU under heavy traffic.
