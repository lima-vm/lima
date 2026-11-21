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
limactl start --tty=false --name k8s --timeout 15m template:k8s
export KUBECONFIG=$(limactl list k8s --format '{{.Dir}}/copied-from-guest/kubeconfig.yaml')
kubectl get nodes
```

`template:k3s` writes the same host path, so the second line is unchanged.

### Pinning the Kubernetes version

`template:k8s` declares the parameters for this, so a pinned cluster needs no custom template.
Two of them move together: `version` selects the control plane, and `release` selects the APT
repository that provides kubeadm, kubelet and kubectl. Pin both to the same minor:

```bash
limactl start --tty=false --name team --param release=v1.33 --param version=v1.33.0 template:k8s
```

Pinning `version` alone leaves the repository on the template's own default minor, so APT
installs components that do not match the control plane. The coupling lives in the template's
provision script, which is the thing to read before pinning anything version-specific:

```bash
limactl template copy template:k8s - | grep -n 'PARAM_release\|PARAM_version'
```

That generalises: a stock template's parameters are defined by the template, not by this file.
List what one declares before guessing at a name.

| Template | Parameters it declares |
|----------|------------------------|
| `k8s` | `version` and `release` (coupled, above), `stability`, `flannelVersion`, `proxy`, `mirror`, plus join parameters |
| `k3s` | `proxy`, `mirror`, `url`, `token`. **No version parameter**, so pinning k3s does need a derived template |

`--param` fails outright on a name the template does not declare, so list them before
guessing: `limactl template yq template:k8s '.param'`. For writing your own, see
[templates.md](templates.md).

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

The port-forwarding row above is the common case of a general Lima rule: the hostagent owns port
forwarding from outside the guest, so a reboot from inside leaves it behind. The rule, the
restart that fixes it, and the distro-independent upgrade steps are in
[networking.md](networking.md) and [instances.md](instances.md).

## Teardown

```bash
limactl stop k8s && limactl delete k8s   # delete needs a stopped instance; no confirmation prompt
```

The cluster, its images and its kubeconfig go with the instance. Unset `KUBECONFIG` afterwards
so later commands do not pick up a stale path.
