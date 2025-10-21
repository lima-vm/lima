---
title: Kubernetes
weight: 4
---

## Single-node

{{< tabpane text=true >}}
{{% tab header="kubeadm" %}}
```bash
limactl start template://k8s
export KUBECONFIG=$(limactl list k8s --format 'unix://{{.Dir}}/copied-from-guest/kubeconfig.yaml')
kubectl create deployment nginx --image nginx:alpine
kubectl create service nodeport nginx --node-port=31080 --tcp=80:80
```

Modify [`templates/k8s.yaml`](https://github.com/lima-vm/lima/blob/master/templates/k8s.yaml) to change
the kubeadm configuration.

See also <https://kubernetes.io/docs/reference/setup-tools/kubeadm/>.
{{% /tab %}}
{{% tab header="k3s" %}}
```bash
limactl start template://k3s
export KUBECONFIG=$(limactl list k3s --format 'unix://{{.Dir}}/copied-from-guest/kubeconfig.yaml')
kubectl create deployment nginx --image nginx:alpine
kubectl create service nodeport nginx --node-port=31080 --tcp=80:80
```

See also <https://docs.k3s.io>.
{{% /tab %}}
{{% tab header="k0s" %}}
```bash
limactl start template://k0s
export KUBECONFIG=$(limactl list k0s --format 'unix://{{.Dir}}/copied-from-guest/kubeconfig.yaml')
kubectl create deployment nginx --image nginx:alpine
kubectl create service nodeport nginx --node-port=31080 --tcp=80:80
```

See also <https://docs.k0sproject.io/>.
{{% /tab %}}
{{% tab header="Usernetes" %}}
```bash
limactl start template://experimental/u7s
export KUBECONFIG=$(limactl list u7s --format 'unix://{{.Dir}}/copied-from-guest/kubeconfig.yaml')
kubectl create deployment nginx --image nginx:alpine
# NodePorts are not available by default in the case of Usernetes
kubectl port-forward deployments/nginx 8080:80
```

See also <https://github.com/rootless-containers/usernetes>.
{{% /tab %}}
{{< /tabpane >}}

## Multi-node

A multi-node cluster can be created by creating multiple VMs connected via the [`lima:user-v2`](../../../config/network/user-v2.md) network.

```bash
limactl create --name k8s-0 --network lima:user-v2
limactl create --name k8s-1 --network lima:user-v2
```

The cluster has to be set up manually, as the built-in templates do not support multi-node mode yet.
Support for multi-node template is tracked in <https://github.com/lima-vm/lima/issues/4100>.