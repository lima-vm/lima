---
title: Krunkit
weight: 4
---

> Warning
> "krunkit" is experimental

| ⚡ Requirement | Lima >= 2.0, macOS >= 13 (Ventura+), Apple Silicon (arm64) |
| ------------- | ----------------------------------------------------------- |

Krunkit runs super‑light VMs on macOS/ARM64 with a focus on GPU access. It builds on [libkrun](https://github.com/containers/libkrun), a library that embeds a VMM so apps can launch processes in a hardware‑isolated VM (HVF on macOS, KVM on Linux). The standout feature is GPU support in the guest via Mesa’s Venus Vulkan driver ([venus](https://docs.mesa3d.org/drivers/venus.html)), enabling Vulkan workloads inside the VM. See the project: [containers/krunkit](https://github.com/containers/krunkit).

## Install krunkit (host)
```bash
brew tap slp/krunkit
brew install krunkit
```
For reference: https://github.com/slp/homebrew-krun


## Using the driver with Lima
Build the driver binary and point Lima to it. See also [Virtual Machine Drivers](../../dev/drivers).

```bash
git clone https://github.com/lima-vm/lima && cd lima

# From the Lima source tree
# <PREFIX> is your installation prefix. With Homebrew, use: $(brew --prefix)
go build -o <PREFIX>/libexec/lima/lima-driver-krunkit ./cmd/lima-driver-krunkit/main_darwin_arm64.go
limactl info   # "vmTypes" should include "krunkit"
```


## Quick start

- Non‑GPU (general workloads)
```bash
limactl start default --vm-type=krunkit
```

- GPU (Vulkan via Venus)
  - Recommended distro: Fedora 40+ (smoothest Mesa/Vulkan setup; uses COPR “slp/mesa-krunkit” for patched mesa-vulkan-drivers).
  - Start from the krunkit template and follow the logs to complete GPU setup.

{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
# GPU (Vulkan via Venus on Fedora)
limactl start template:experimental/krunkit
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
vmType: krunkit

# For AI workloads, at least 4GiB memory and 4 CPUs are recommended.
memory: 4GiB
cpus: 4
arch: aarch64

# Fedora 40+ is preferred for Mesa & Vulkan (Venus) support
base:
- template://_images/fedora

mounts:
- location: "~"
  writable: true

mountType: virtiofs

vmOpts:
  krunkit:
    gpuAccel: true
```
{{% /tab %}}
{{< /tabpane >}}

After the VM is READY, inside the VM:
```bash
sudo install-vulkan-gpu.sh
```

## Notes and caveats
- macOS Ventura or later on Apple Silicon is required.
- GPU mode requires a Fedora image/template; Fedora 40+ recommended for Mesa/Vulkan (Venus).
- To verify GPU/Vulkan in the guest, use tools like `vulkaninfo` after running the install script.
- `Libkrun` and [`Ramalama`](https://github.com/containers/ramalama)(a tool that simplifies running AI models locally) use CPU inferencing as of **July 2, 2025** and are actively working to support GPU inferencing. [More info](https://developers.redhat.com/articles/2025/07/02/supercharging-ai-isolation-microvms-ramalama-libkrun#current_limitations_and_future_directions__gpu_enablement).
- Driver architecture details: see [Virtual Machine Drivers](../../dev/drivers).