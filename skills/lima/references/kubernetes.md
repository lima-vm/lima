# Kubernetes

A single-node cluster in a VM, driven with `kubectl` from the host. Reference:
<https://lima-vm.io/docs/examples/containers/kubernetes/>.

## Decide

| You need | Template |
|----------|----------|
| The lightest cluster that works | `template:k3s` |
| An upstream kubeadm cluster | `template:k8s` |
| k0s | `template:k0s` |

`k8s` is Tier 1 at <https://lima-vm.io/docs/templates/>; `k3s` and `k0s` are tested less
often. Prefer `k8s` when the task must not flake, `k3s` when start-up time matters. If the
user named a Tier 2 template, say so rather than silently substituting.

## Do

Both templates copy a kubeconfig back to the host when the cluster is ready:

```bash
limactl start -y --name k8s --timeout 15m template:k8s
export KUBECONFIG=$(limactl list k8s --format '{{.Dir}}/copied-from-guest/kubeconfig.yaml')
kubectl get nodes
```

`template:k3s` writes the same host path, so the second line is unchanged.

The file appears only after the cluster is up. The copy rule is marked `skipIfMissing`, so an
instance that started but whose cluster failed will leave no kubeconfig and no error: check for
the file before assuming the cluster is healthy.

## Verify

```bash
test -f "$KUBECONFIG" || echo "cluster did not come up"
kubectl get nodes -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}'
```

## When it breaks

| Signature | Cause | Fix |
|-----------|-------|-----|
| No kubeconfig on the host | the cluster never became ready | `limactl shell k3s sudo journalctl -u k3s`, or for kubeadm check `/var/log/cloud-init-output.log` in the guest |
| `kubectl` cannot reach the API server | the forwarded port is down | `limactl list <inst> --format json`, then `ha.stderr.log` |
| **The API server stops responding after an in-guest reboot or OS upgrade** | rebooting from inside the guest does not re-establish host port forwarding, even though the cluster is healthy | restart from the host: `limactl restart <inst>`. In automation, never reboot in-guest |
| Pods cannot reach a service on the host | the guest sees the host at `192.168.5.2` under user-mode networking | point workloads at `192.168.5.2`, and bind the host service on loopback. See [networking.md](networking.md) |
| The cluster runs but is unusably slow | default CPU and memory | recreate with `--cpus` and `--memory` |

## Upgrading the guest OS

This is the common case of the port-forwarding row above. `limactl restart` stops the instance
gracefully and starts it again, so it is a full power cycle: the upgraded kernel is picked up
*and* the port forwards come back. Upgrade in-guest without rebooting, then restart from the
host.

Do not guess the guest's package manager. The templates do not all use the same distribution,
so read it first:

```bash
limactl shell <inst> cat /etc/os-release
```

## Teardown

```bash
limactl delete -y k8s
```

The cluster, its images and its kubeconfig go with the instance. Unset `KUBECONFIG` afterwards
so later commands do not pick up a stale path.
