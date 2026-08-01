---
title: Multi-node
weight: 20
---

A multi-node cluster can be created by creating multiple VMs connected via the [`lima:user-v2`](../../../config/network/user-v2.md) network.

The following templates are designed to support multi-node mode:
- `k8s` (since Lima v2.0)
- `k3s` (since Lima v2.1)

{{< tabpane text=true >}}
{{% tab header="Lima v2.0" %}}
```bash
limactl start --name k8s --network lima:user-v2 template:k8s
limactl shell k8s sudo kubeadm token create --print-join-command
# (The join command printed here)
```

```bash
limactl start --name k8s-1 --network lima:user-v2 template:k8s
limactl shell k8s-1 sudo bash -euxc "kubeadm reset --force ; ip link delete cni0 ; ip link delete flannel.1 ; rm -rf /var/lib/cni /etc/cni"
limactl shell k8s-1 sudo <JOIN_COMMAND_FROM_ABOVE>
```
{{% /tab %}}
{{% tab header="Lima v2.1 (k8s)" %}}
```bash
limactl start --name k8s --network lima:user-v2 template:k8s
limactl shell k8s sudo kubeadm token create --print-join-command
# (The parameters for the start command printed here)
```

```bash
limactl start --name k8s-1 --network lima:user-v2 template:k8s \
              --param url="https://<ADDRESS_FROM_ABOVE>" --param token="<TOKEN_FROM_ABOVE>" \
              --param discoveryTokenCaCertHash="<DISCOVERY_TOKEN_CA_CERT_HASH_FROM_ABOVE>"
```
{{% /tab %}}
{{% tab header="Lima v2.1 (k3s)" %}}
```bash
limactl start --name k3s --network lima:user-v2 template:k3s
printf "https://lima-%s.internal:6443\n" k3s
# (The url for the start command printed here)
limactl shell k3s sudo cat /var/lib/rancher/k3s/server/node-token
# (The token for the start command printed here)
```

```bash
limactl start --name k3s-1 --network lima:user-v2 template:k3s \
              --param url="<URL_FROM_ABOVE>" --param token="<TOKEN_FROM_ABOVE>"
```
{{% /tab %}}
{{< /tabpane >}}
