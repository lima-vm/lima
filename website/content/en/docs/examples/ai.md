---
title: AI agents
weight: 5
---

Lima is useful for running AI agents inside a VM, so as to prevent agents
from directly reading, writing, or executing the host files.

For running AI agents, it is highly recommended to only mount your project directory (current directory)
into the VM:

{{< tabpane text=true >}}
{{% tab header="Lima v2.0+" %}}
```bash
limactl start --mount-only .:w
```

Drop `:w` for read-only mode.
{{% /tab %}}
{{% tab header="Lima v1.x" %}}
```bash
limactl start --set ".mounts=[{\"location\":\"$(pwd)\", \"writable\":true}]"
```

Set `writable` to `false` for read-only mode.
{{% /tab %}}
{{< /tabpane >}}

<!--
AI agents are sorted alphabetically.

Node.js is installed via snap, not apt, as AI agents often require a very recent release of Node.js.
-->
{{< tabpane text=true >}}
{{% tab header="Aider" %}}
```
lima sudo apt install -y pipx
lima pipx install aider-install
lima sh -c 'echo "export PATH=$PATH:$HOME/.local/bin" >>~/.bash_profile'
lima aider-install
lima aider
```

Follow the guide shown in the first session for authentication.

Alternatively, you can set environmental variables via:
```
lima vi "/home/${USER}.linux/.bash_profile"
```

See also <https://github.com/Aider-AI/aider>.
{{% /tab %}}
{{% tab header="Claude Code" %}}
```
lima sudo snap install node --classic
lima sudo npm install -g @anthropic-ai/claude-code
lima claude
```

Follow the guide shown in the first session for authentication.

Alternatively, you can set `export ANTHROPIC_API_KEY...` via:
```
lima vi "/home/${USER}.linux/.bash_profile"
```

See also <https://github.com/anthropics/claude-code>.
{{% /tab %}}
{{% tab header="Codex" %}}
```
lima sudo snap install node --classic
lima sudo npm install -g @openai/codex
lima codex
```

Follow the guide shown in the first session for authentication.

Alternatively, you can set `export OPENAI_API_KEY...` via:
```
lima vi "/home/${USER}.linux/.bash_profile"
```

See also <https://github.com/openai/codex>.
{{% /tab %}}
{{% tab header="Gemini" %}}
```
lima sudo snap install node --classic
lima sudo npm install -g @google/gemini-cli
lima gemini
```

Follow the guide shown in the first session for authentication.

Alternatively, you can set `export GEMINI_API_KEY...` via:
```
lima vi "/home/${USER}.linux/.bash_profile"
```

See also <https://github.com/google-gemini/gemini-cli>.
{{% /tab %}}
{{% tab header="GitHub Copilot" %}}
```
lima sudo snap install node --classic
lima sudo npm install -g @github/copilot
lima copilot
```

Type `/login` in the first session for authentication.

Alternatively, you can set `export GH_TOKEN=...` via:
```
lima vi "/home/${USER}.linux/.bash_profile"
```

See also <https://github.com/github/copilot-cli>.
{{% /tab %}}
{{< /tabpane >}}

## See also

- [Config » AI](../config/ai/)
