---
title: Examples
weight: 3
---

## Running Linux commands
```bash
lima sudo apt-get install -y neofetch
lima neofetch
```

## Accessing host files

By default, the VM has read-only accesses to `/Users/<USERNAME>` and read-write accesses to `/tmp/lima`.

To allow writing to `/Users/<USERNAME>`:
```bash
limactl edit --mount-writable --mount-type=virtiofs
```

Specifying `--mount-type=virtiofs` is not necessary here, but it is highly recommended
for the best performance and stability.

## Running containers
{{< tabpane text=true >}}

{{% tab header="containerd" %}}
```bash
nerdctl.lima run -d --name nginx -p 127.0.0.1:8080:80 nginx:alpine
```
{{% /tab %}}

{{% tab header="Docker" %}}
```bash
limactl start template://docker
export DOCKER_HOST=$(limactl list docker --format 'unix://{{.Dir}}/sock/docker.sock')
docker run -d --name nginx -p 127.0.0.1:8080:80 nginx:alpine
```
{{% /tab %}}

{{% tab header="Podman" %}}
```bash
limactl start template://podman
export DOCKER_HOST=$(limactl list podman --format 'unix://{{.Dir}}/sock/podman.sock')
docker run -d --name nginx -p 127.0.0.1:8080:80 nginx:alpine
```
{{% /tab %}}

{{% tab header="Kubernetes" %}}
```bash
limactl start template://k8s
export KUBECONFIG=$(limactl list k8s --format 'unix://{{.Dir}}/copied-from-guest/kubeconfig.yaml')
kubectl apply -f ...
```
{{% /tab %}}

{{< /tabpane >}}

- <http://127.0.0.1:8080> is accessible from the host, as well as from the VM.

- For the usage of containerd and nerdctl (contaiNERD ctl), visit <https://github.com/containerd/containerd>
and <https://github.com/containerd/nerdctl>.

- If you have installed Lima by `make install`, the `nerdctl.lima` command is also available as `nerdctl`.
  If you have installed Lima by `brew install lima`, you may make an alias (or a symlink) by yourself:
  `alias nerdctl=nerdctl.lima`

## Advanced configuration

```bash
limactl start \
  --name=default \
  --cpus=4 \
  --memory=8 \
  --vm-type=vz \
  --rosetta \
  --mount-type=virtiofs \
  --mount-writable \
  --network=vzNAT \
  template://fedora
```

- `--name=default`: Set the instance name to "default"
- `--cpus=4`: Set the number of the CPUs to 4
- `--memory=8`: Set the amount of the memory to 8 GiB
- `--vm-type=vz`: Use Apple's Virtualization.framework (vz) to enable Rosetta, virtiofs, and vzNAT
- `--rosetta`: Allow running Intel (AMD) binaries on ARM
- `--mount-type=virtiofs`: Use virtiofs for better performance
- `--mount-writable`: Make the home mount (`/Users/<USERNAME>`) writable
- `--network=vzNAT`: Make the VM reachable from the host by its IP address
- `template://fedora`: Use Fedora

## Configure Existing VM
Use [limactl edit](../reference/limactl_edit) to configure a VM instance, like adjusting the disk size, CPUs, or memory.
For now, the VM must be stopped before updating its configuration.
```bash
limactl stop default

# Edit value in the YAML file
limactl edit default

# Edit using flags
limactl edit default --cpus 2 
```
