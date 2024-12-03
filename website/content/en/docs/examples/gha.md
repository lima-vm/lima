---
title: GitHub Actions
weight: 10
---

## Running Lima on GitHub Actions

On GitHub Actions, Lima is useful for:
- Running commands on non-Ubuntu operating systems (e.g., Fedora for testing SELinux)
- Emulating multiple hosts

While these tasks can be partially accomplished with containers like Docker, those containers still rely on the Ubuntu host's kernel and cannot utilize features missing in Ubuntu, such as SELinux.

In contrast, Lima runs virtual machines that do not depend on the Ubuntu host's kernel.

The following GitHub Actions workflow illustrates how to run multiple instances of Fedora using Lima.
The instances are connected by the `user-v2` network.

```yaml
name: Fedora

on:
  workflow_dispatch:
  pull_request:

jobs:
  fedora:
    runs-on: ubuntu-24.04
    steps:
    - name: Check out code
      uses: actions/checkout@v4

    - name: "Install QEMU"
      run: |
        set -eux
        sudo apt-get update
        sudo apt-get install -y --no-install-recommends ovmf qemu-system-x86 qemu-utils
        sudo modprobe kvm
        # `sudo usermod -aG kvm $(whoami)` does not take an effect on GHA
        sudo chown $(whoami) /dev/kvm

    - name: "Install Lima"
      run: |
        set -eux
        LIMA_VERSION=$(curl -fsSL https://api.github.com/repos/lima-vm/lima/releases/latest | jq -r .tag_name)
        curl -fsSL https://github.com/lima-vm/lima/releases/download/${LIMA_VERSION}/lima-${LIMA_VERSION:1}-Linux-x86_64.tar.gz | sudo tar Cxzvf /usr/local -

    - name: "Cache ~/.cache/lima"
      uses: actions/cache@v4
      with:
        path: ~/.cache/lima
        key: lima-${{ env.LIMA_VERSION }}

    - name: "Start an instance of Fedora"
      run: |
        set -eux
        limactl start --name=default --cpus=1 --memory=1 --network=lima:user-v2 template://fedora
        lima sudo dnf install -y httpd
        lima sudo systemctl enable --now httpd

    - name: "Start another instance of Fedora"
      run: |
        set -eux
        limactl start --name=another --cpus=1 --memory=1 --network=lima:user-v2 template://fedora
        limactl shell another curl http://lima-default.internal
```

### Plain mode

The `--plain` mode is useful when you want the VM instance to be as close as possible to a physical host:

```yaml
    - name: "Start Fedora"
      # --plain is set to disable file sharing, port forwarding, built-in containerd, etc.
      run: limactl start --plain --name=default --cpus=1 --memory=1 --network=lima:user-v2 template://fedora

    - name: "Initialize Fedora"
      # plain old rsync and ssh are used for the initialization of the guest,
      # so that people who are not familiar with Lima can understand the initialization steps.
      run: |
        set -eux -o pipefail
        # Initialize SSH
        mkdir -p -m 0700 ~/.ssh
        cat ~/.lima/default/ssh.config >> ~/.ssh/config
        # Sync the current directory to /tmp/repo in the guest
        rsync -a -e ssh . lima-default:/tmp/repo
        # Install packages
        ssh lima-default sudo dnf install -y httpd
```

### Full examples
- https://github.com/kubernetes-sigs/kind/blob/v0.25.0/.github/workflows/vm.yaml#L47-L84
