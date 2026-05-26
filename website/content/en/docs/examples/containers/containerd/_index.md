---
title: containerd (Default)
weight: 1
---

Lima comes with the built-in integration for [containerd](https://containerd.io) and
[nerdctl](https://github.com/containerd/nerdctl) (contaiNERD CTL):

{{< tabpane text=true >}}
{{% tab header="Rootless" %}}
```bash
lima nerdctl run -d --name nginx -p 127.0.0.1:8080:80 nginx:alpine
```

or

```bash
nerdctl.lima run -d --name nginx -p 127.0.0.1:8080:80 nginx:alpine
```

- If you have installed Lima by [`make install`](../../../installation/source.md), the `nerdctl.lima` command is also available as `nerdctl`.
- If you have installed Lima by [`brew install lima`](../../../installation/_index.md), you may make an alias (or a symlink) by yourself:
  `alias nerdctl=nerdctl.lima`
{{% /tab %}}
{{% tab header="Rootful" %}}
```bash
limactl start --containerd=system
lima sudo nerdctl run -d --name nginx -p 127.0.0.1:8080:80 nginx:alpine
```
{{% /tab %}}
{{< /tabpane >}}

The usage of the `nerdctl` command is similar to the `docker` command. See the [Command Reference](https://github.com/containerd/nerdctl/blob/main/docs/command-reference.md).

## Disabling containerd

To disable containerd, start an instance with `--containerd=none`:

```bash
limactl start --containerd=none
```
