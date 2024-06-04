[[🌎**Web site**]](https://lima-vm.io/)
[[📖**Documentation**]](https://lima-vm.io/docs/)
[[👤**Slack (`#lima`)**]](https://slack.cncf.io)

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="website/static/images/logo-dark.svg">
  <img alt="Shows a stylized 'Lima' text in bold, modern font" src="website/static/images/logo.svg" width=400 />
</picture>

# Lima: Linux Machines

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/6505/badge)](https://www.bestpractices.dev/projects/6505)

[Lima](https://lima-vm.io/) launches Linux virtual machines with automatic file sharing and port forwarding (similar to WSL2).

The original goal of Lima was to promote [containerd](https://containerd.io) including [nerdctl (contaiNERD ctl)](https://github.com/containerd/nerdctl)
to Mac users, but Lima can be used for non-container applications as well.

Lima also supports other container engines (Docker, Podman, Kubernetes, etc.) and non-macOS hosts (Linux, NetBSD, etc.).

## Getting started
Set up (on macOS):
```bash
brew install lima
limactl start
```

To run Linux commands:
```bash
lima sudo apt-get install -y neofetch
lima neofetch
```

To run containers with containerd:
```bash
lima nerdctl run --rm hello-world
```

To run containers with Docker:
```bash
limactl start template://docker
export DOCKER_HOST=$(limactl list docker --format 'unix://{{.Dir}}/sock/docker.sock')
docker run --rm hello-world
```

To run containers with Kubernetes:
```bash
limactl start template://k8s
export KUBECONFIG=$(limactl list k8s --format 'unix://{{.Dir}}/copied-from-guest/kubeconfig.yaml')
kubectl apply -f ...
```

See <https://lima-vm.io/docs/> for the further information.

## Community
### Adopters

Container environments:
- [Rancher Desktop](https://rancherdesktop.io/): Kubernetes and container management to the desktop
- [Colima](https://github.com/abiosoft/colima): Docker (and Kubernetes) on macOS with minimal setup
- [Finch](https://github.com/runfinch/finch): Finch is a command line client for local container development
- [Podman Desktop](https://podman-desktop.io/): Podman Desktop GUI has a plug-in for Lima virtual machines

GUI:
- [Lima xbar plugin](https://github.com/unixorn/lima-xbar-plugin): [xbar](https://xbarapp.com/) plugin to start/stop VMs from the menu bar and see their running status.
- [lima-gui](https://github.com/afbjorklund/lima-gui): Qt GUI for Lima

### Communication channels
- [GitHub Discussions](https://github.com/lima-vm/lima/discussions)
- `#lima` channel in the CNCF Slack
  - New account: <https://slack.cncf.io/>
  - Login: <https://cloud-native.slack.com/>

### Code of Conduct
Lima follows the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md).

- - -
**We are a [Cloud Native Computing Foundation](https://cncf.io/) sandbox project.**

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://www.cncf.io/wp-content/uploads/2022/07/cncf-white-logo.svg">
  <img src="https://www.cncf.io/wp-content/uploads/2022/07/cncf-color-bg.svg" width=300 />
</picture>

The Linux Foundation® (TLF) has registered trademarks and uses trademarks. For a list of TLF trademarks, see [Trademark Usage](https://www.linuxfoundation.org/trademark-usage/).
