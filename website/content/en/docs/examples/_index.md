---
title: Examples
weight: 3
---

## Running Linux commands
```bash
lima uname -a
```

## Accessing host files

By default, the VM has read-only accesses to `/Users/<USERNAME>`.

To allow writing to `/Users/<USERNAME>`:
```bash
limactl edit --mount-writable
```

{{% alert title="Hint" color=success %}}
Lima prior to v2.0 mounts `/tmp/lima` too in read-write mode.

This directory is no longer mounted by default since Lima v2.0.
To mount `/tmp/lima` in Lima v2.0 and later, set `--mount /tmp/lima:w`.
The `:w` suffix indicates read-write mode.
{{% /alert %}}

## Running containers
{{< tabpane text=true >}}

{{% tab header="containerd" %}}
```bash
nerdctl.lima run -d --name nginx -p 127.0.0.1:8080:80 nginx:alpine
```
{{% /tab %}}

{{% tab header="Docker" %}}
```bash
limactl start template:docker
export DOCKER_HOST=$(limactl list docker --format 'unix://{{.Dir}}/sock/docker.sock')
docker run -d --name nginx -p 127.0.0.1:8080:80 nginx:alpine
```
{{% /tab %}}

{{% tab header="Podman" %}}
```bash
limactl start template:podman
export DOCKER_HOST=$(limactl list podman --format 'unix://{{.Dir}}/sock/podman.sock')
docker run -d --name nginx -p 127.0.0.1:8080:80 nginx:alpine
```
{{% /tab %}}

{{% tab header="Kubernetes" %}}
```bash
limactl start template:k8s
export KUBECONFIG=$(limactl list k8s --format 'unix://{{.Dir}}/copied-from-guest/kubeconfig.yaml')
kubectl create deployment nginx --image nginx:alpine
kubectl create service nodeport nginx --node-port=31080 --tcp=80:80
```
{{% /tab %}}

{{< /tabpane >}}

- <http://127.0.0.1:8080> is accessible from the host, as well as from the VM.

- See more [examples](./containers/).

## Advanced configuration

```bash
limactl start \
  --name=default \
  --cpus=4 \
  --memory=8 \
  --vm-type=vz \
  --rosetta \
  --mount-writable \
  --network=vzNAT \
  template:fedora
```

- `--name=default`: Set the instance name to "default"
- `--cpus=4`: Set the number of the CPUs to 4
- `--memory=8`: Set the amount of the memory to 8 GiB
- `--vm-type=vz`: Use Apple's Virtualization.framework (vz) to enable Rosetta, virtiofs, and vzNAT
- `--rosetta`: Allow running Intel (AMD) binaries on ARM
- `--mount-writable`: Make the home mount (`/Users/<USERNAME>`) writable
- `--network=vzNAT`: Make the VM reachable from the host by its IP address
- `template:fedora`: Use Fedora
