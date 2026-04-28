---
title: Visual Studio Code
weight: 9
---

## Securing Visual Studio Code with Lima

Lima helps securing the development environment by running it inside a VM.
Notably, this prevents AI agents, such as [GitHub Copilot in VS Code](https://code.visualstudio.com/docs/copilot/overview), from directly executing untrusted commands on the host.

1. Start a Lima instance. If you use GitHub Copilot, consider disabling mounts by passing the `--mount-none` flag to prevent Copilot from accessing host files:

```bash
limactl start --mount-none
```

2. Add the following line to `~/.ssh/config`:

```
Include ~/.lima/*/ssh.config
```

3. Open the Remote Explorer in the Visual Studio Code sidebar and select `lima-<INSTANCE>` from the SSH remote list.

![](/images/vscode-remote-explorer.png)

{{% alert title="Hint" color=success %}}
If the Remote Explorer is missing in the sidebar, install the following extensions:
- [Remote Explorer](https://marketplace.visualstudio.com/items?itemName=ms-vscode.remote-explorer)
- [Remote - SSH](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-ssh)

See also the [documentation](https://code.visualstudio.com/docs/remote/ssh) of Visual Studio Code for further troubleshooting.
{{% /alert %}}

4. Set up the workspace by clicking `Clone Git Repository...` on the `Welcome` screen, or copy the project directory with `limactl cp`:

```bash
limactl cp -r DIR default:~/
```
