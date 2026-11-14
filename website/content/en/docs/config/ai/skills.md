---
title: Agent Skills
weight: 30
---

| ⚡ Requirement | Lima >= 2.0 |
|---------------|-------------|

Lima ships [Agent Skills](https://agentskills.io/) in the
[`skills/`](https://github.com/lima-vm/lima/tree/master/skills) directory of the repository.
A skill is a Markdown file that teaches an AI coding agent how to use Lima: which template to
start from, how to run `limactl` without a terminal, how to expose a container socket, and
what to read when something fails.

Two skills are provided:

- `lima`: creating and operating instances, containers, Kubernetes, networking, mounts,
  templates and troubleshooting.
- `lima-agent-sandbox`: running an AI coding agent inside a VM, and connecting a client on the
  host to a VM over MCP. The `limactl shell --sync` workflow it describes requires Lima >= 2.1,
  and `--sync-exclude` and `.limasyncignore` require Lima >= 2.3.

## Installing

Any tool that implements the Agent Skills standard can install them. For example:

```bash
npx skills add lima-vm/lima
```

Add `-g` to install for the current user rather than the current project.

Alternatively, copy the directories from the repository into the location your agent reads
skills from.

## See also

- [AI agents inside Lima]({{< ref "/docs/config/ai/inside" >}})
- [AI agents outside Lima (MCP)]({{< ref "/docs/config/ai/outside" >}})
- [Examples » AI agents]({{< ref "/docs/examples/ai" >}})
