---
title: Single-node
weight: 10
---

{{< tabpane text=true >}}
{{% tab header="kubeadm" %}}
```bash
limactl start template:k8s
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
limactl start template:k3s
export KUBECONFIG=$(limactl list k3s --format 'unix://{{.Dir}}/copied-from-guest/kubeconfig.yaml')
kubectl create deployment nginx --image nginx:alpine
kubectl create service nodeport nginx --node-port=31080 --tcp=80:80
```

See also <https://docs.k3s.io>.
{{% /tab %}}
{{% tab header="k0s" %}}
```bash
limactl start template:k0s
export KUBECONFIG=$(limactl list k0s --format 'unix://{{.Dir}}/copied-from-guest/kubeconfig.yaml')
kubectl create deployment nginx --image nginx:alpine
kubectl create service nodeport nginx --node-port=31080 --tcp=80:80
```

See also <https://docs.k0sproject.io/>.
{{% /tab %}}
{{% tab header="RKE2" %}}
```bash
limactl start template:experimental/rke2
export KUBECONFIG=$(limactl list rke2 --format 'unix://{{.Dir}}/copied-from-guest/kubeconfig.yaml')
kubectl create deployment nginx --image nginx:alpine
kubectl create service nodeport nginx --node-port=31080 --tcp=80:80
```

See also <https://docs.rke2.io/>.
{{% /tab %}}
{{% tab header="Usernetes" %}}
```bash
limactl start template:experimental/u7s
export KUBECONFIG=$(limactl list u7s --format 'unix://{{.Dir}}/copied-from-guest/kubeconfig.yaml')
kubectl create deployment nginx --image nginx:alpine
# NodePorts are not available by default in the case of Usernetes
kubectl port-forward deployments/nginx 8080:80
```

See also <https://github.com/rootless-containers/usernetes>.
{{% /tab %}}
{{< /tabpane >}}
