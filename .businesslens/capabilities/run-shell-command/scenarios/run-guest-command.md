---
kind: primary
routes:
  agent: MCP
steps:
  - text: The AI agent calls `run_shell_command` with a command and an absolute directory
    kind: actor
    actor: ai-agent
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      agent:
        place: mcp
  - text: The Product runs the command in the guest and returns its standard output, standard error and exit code
    kind: product
    actor: ai-agent
    entities:
      - {entity: instance, effect: reads, facts: [SSH local port]}
    contexts:
      agent:
        place: mcp
---

# Run a command in the guest

## Trigger

The AI agent needs to run a program in the instance.

## Outcome

The AI agent has the command's result.

## Edge cases

- A command that fails still returns its output with its exit code.
