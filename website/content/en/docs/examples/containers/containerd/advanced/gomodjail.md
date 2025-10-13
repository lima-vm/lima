---
title: Enhanced supply chain security with gomodjail
linkTitle: gomodjail
weight: 1
---

[gomodjail](https://github.com/AkihiroSuda/gomodjail) is an experimental library sandbox for Go modules.

gomodjail imposes syscall restrictions on a specific set of Go modules, so as to mitigate their potential vulnerabilities and supply chain attack vectors.
A restricted module is hindered to access files and execute commands.

gomodjail can be enabled for nerdctl by using the `nerdctl.gomodjail` binary.

```bash
lima nerdctl.gomodjail ...
```

For the gomodjail policy applied to `nerdctl.gomodjail`, see <https://github.com/containerd/nerdctl/blob/main/go.mod>.
