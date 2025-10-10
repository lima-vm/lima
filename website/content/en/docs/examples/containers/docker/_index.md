---
title: Docker
weight: 2
---

{{< tabpane text=true >}}
{{% tab header="Rootless" %}}
```bash
limactl start template://docker
export DOCKER_HOST=$(limactl list docker --format 'unix://{{.Dir}}/sock/docker.sock')
docker run -d --name nginx -p 127.0.0.1:8080:80 nginx:alpine
```
{{% /tab %}}
{{% tab header="Rootful" %}}
```bash
limactl start template://docker-rootful
export DOCKER_HOST=$(limactl list docker-rootful --format 'unix://{{.Dir}}/sock/docker.sock')
docker run -d --name nginx -p 127.0.0.1:8080:80 nginx:alpine
```
{{% /tab %}}
{{< /tabpane >}}
