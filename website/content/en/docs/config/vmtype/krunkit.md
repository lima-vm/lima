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

Start a krunkit VM with rootful containerd (required for granting containers access to `/dev/dri` and the Vulkan API):

{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start default --vm-type=krunkit --containerd=system
limactl shell default
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
vmType: krunkit

base:
- template://_images/ubuntu
- template://_default/mounts

containerd:
  system: true
```
{{% /tab %}}
{{< /tabpane >}}

Run AI models with [`llama.cpp`](https://github.com/ggml-org/llama.cpp) in a container using `nerdctl`:

1) Place a `.gguf` model inside the VM, e.g. `~/models/YourModel.gguf` or download it from [Hugging Face](https://huggingface.co/models?library=gguf)

2) Launch the container with GPU device nodes and bind‑mount the model directory:

```bash
sudo nerdctl run --rm -ti \
  --device /dev/dri \
  -v ~/models:/models \
  quay.io/slopezpa/fedora-vgpu-llama
```

3) Inside the container, run llama.cpp:

```bash
llama-cli -m /models/YourModel.gguf -b 512 -ngl 99 -p "Introduce yourself"
```

## Notes and caveats
- macOS Ventura or later on Apple Silicon is required.
- Rootful containerd (`--containerd=system`) is necessary to pass through /dev/dri to containers.
- To verify GPU/Vulkan in the guest container, use tools like `vulkaninfo`.
- Driver architecture details: see [Virtual Machine Drivers](../../dev/drivers).