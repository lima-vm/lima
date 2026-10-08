---
appliesTo:
  - type: context
    context:
      place: limactl
references:
  - kind: code
    role: implementation
    target: pkg/yqutil/yqutil.go
  - kind: code
    role: implementation
    target: cmd/limactl/yq_restrictions.go
---

# Configuration expressions cannot read the host's environment or files

The yq expressions given with `--set`, `--yq`, `--filter` and `limactl template
yq` run with environment and file operators switched off.
