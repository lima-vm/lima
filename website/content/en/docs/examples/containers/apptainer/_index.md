---
title: Apptainer
weight: 90
---

{{< tabpane text=true >}}
{{% tab header="Rootless" %}}
```bash
limactl start template://apptainer
limactl shell apptainer apptainer run -u -B $HOME:$HOME docker://alpine
```
{{% /tab %}}
{{% tab header="Rootful" %}}
```bash
limactl start template://apptainer-rootful
limactl shell apptainer-rootful apptainer run -u -B $HOME:$HOME docker://alpine
```
{{% /tab %}}
{{< /tabpane >}}

See also <https://apptainer.org/docs/user/latest/>.
