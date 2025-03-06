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

    - name: "Set up Lima"
      uses: lima-vm/lima-actions/setup@v1
      id: lima-actions-setup

    - name: "Cache ~/.cache/lima"
      uses: actions/cache@v4
      with:
        path: ~/.cache/lima
        key: lima-${{ steps.lima-actions-setup.outputs.version }}

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

See also <https://github.com/lima-vm/lima-actions>.

### Plain mode

The `--plain` mode is useful when you want the VM instance to be as close as possible to a physical host:

```yaml
    - name: "Start Fedora"
      # --plain is set to disable file sharing, port forwarding, built-in containerd, etc.
      run: limactl start --plain --name=default --cpus=1 --memory=1 --network=lima:user-v2 template://fedora

    - name: "Set up SSH"
      uses: lima-vm/lima-actions/ssh@v1

    - name: "Initialize Fedora"
      # plain old rsync and ssh are used for the initialization of the guest,
      # so that people who are not familiar with Lima can understand the initialization steps.
      run: |
        set -eux -o pipefail
        # Sync the current directory to /tmp/repo in the guest
        rsync -a -e ssh . lima-default:/tmp/repo
        # Install packages
        ssh lima-default sudo dnf install -y httpd
```

### Full examples
- https://github.com/kubernetes-sigs/kind/blob/v0.25.0/.github/workflows/vm.yaml#L47-L84
