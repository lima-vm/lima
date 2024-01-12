---
title: Colima (third-party project)
weight: 0
---

## "How does Lima relate to Colima?"

[Colima](https://github.com/abiosoft/colima) is a third-party project
that wraps Lima to provide an alternative user experience for launching containers.

The key difference is that Colima launches Docker by default,
while Lima launches containerd by default.

| Container  | Lima                              | Colima                              |
|------------|-----------------------------------|-------------------------------------|
| containerd | `limactl start`                   | `colima start --runtime=containerd` |
| Docker     | `limactl start template://docker` | `colima start`                      |
| Podman     | `limactl start template://podman` | -                                   |
| Kubernetes | `limactl start template://k8s`    | `colima start --kubernetes`         |

The `colima` CLI is similar to the `limactl` CLI, but there are subtle differences:

| Configuration      | Lima                                       | Colima                            |
|--------------------|--------------------------------------------|-----------------------------------|
| CPUs               | `limactl start --cpus=4`                   | `colima start --cpu=4`            |
| Reverse SSHFS      | `limactl start --mount-type=reverse-sshfs` | `colima start --mount-type=sshfs` |
| Rosetta            | `limactl start --rosetta`                  | `colima start --vz-rosetta`       |
| Access to VM by IP | `limactl start --network=lima:shared`      | `colima start --network-address`  |
