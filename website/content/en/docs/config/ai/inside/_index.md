---
title: AI agents inside Lima
weight: 10
---

Lima is useful for running AI agents (e.g., Codex, Claude, Gemini) so as to prevent them
from directly reading, writing, or executing the host files.

Lima v2.0 is planned to be released with built-in templates for well-known AI agents.

For Lima v1.x, you can install AI agents in Lima manually.

e.g.,

```bash
lima sudo apt install -y npm
lima sudo npm install -g @google/gemini-cli
lima gemini
```