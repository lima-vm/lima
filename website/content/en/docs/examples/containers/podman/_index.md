---
title: Podman
weight: 3
---

{{< tabpane text=true >}}
{{% tab header="Rootless" %}}
To use `podman` command in the VM:
```bash
limactl start template://podman
limactl shell podman podman run -d --name nginx -p 127.0.0.1:8080:80 docker.io/library/nginx:alpine
```

To use `podman` command on the host:
```bash
export CONTAINER_HOST=$(limactl list podman --format 'unix://{{.Dir}}/sock/podman.sock')
podman --remote run -d --name nginx -p 127.0.0.1:8080:80 docker.io/library/nginx:alpine
```

To use `docker` command on the host:
```bash
export DOCKER_HOST=$(limactl list podman --format 'unix://{{.Dir}}/sock/podman.sock')
docker run -d --name nginx -p 127.0.0.1:8080:80 docker.io/library/nginx:alpine
```
{{% /tab %}}
{{% tab header="Rootful" %}}
To use `podman` command in the VM:
```bash
limactl start template://podman-rootful
limactl shell podman-rootful sudo podman run -d --name nginx -p 127.0.0.1:8080:80 docker.io/library/nginx:alpine
```

To use `podman` command on the host:
```bash
export CONTAINER_HOST=$(limactl list podman-rootful --format 'unix://{{.Dir}}/sock/podman.sock')
podman --remote run -d --name nginx -p 127.0.0.1:8080:80 docker.io/library/nginx:alpine
```

To use `docker` command on the host:
```bash
export DOCKER_HOST=$(limactl list podman-rootful --format 'unix://{{.Dir}}/sock/podman.sock')
docker run -d --name nginx -p 127.0.0.1:8080:80 docker.io/library/nginx:alpine
```
{{% /tab %}}
{{< /tabpane >}}
