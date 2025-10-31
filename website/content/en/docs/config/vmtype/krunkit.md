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

You can run AI models either:
- With containers (fast to get started; any distro works), or
- Without containers (choose Fedora; build `llama.cpp` from source).

Before running, install a small model on the host so examples can run quickly. We’ll use `Qwen3‑1.7B GGUF`:

```bash
mkdir -p models
curl -LO --output-dir models 'https://huggingface.co/Qwen/Qwen3-1.7B-GGUF/resolve/main/Qwen3-1.7B-Q8_0.gguf'
```

### 1) Run models using containers (easiest)

Start a krunkit VM with the default Lima template:

{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --vm-type=krunkit
limactl shell default
```
{{% /tab %}}
{{< /tabpane >}}

Then inside the VM:

```bash
nerdctl run --rm -ti \
  --device /dev/dri \
  -v $(pwd)/models:/models \
  quay.io/slopezpa/fedora-vgpu-llama
```
For reference: https://sinrega.org/2024-03-06-enabling-containers-gpu-macos/

Once inside the container:

```bash
llama-cli -m /models/Qwen3-1.7B-Q8_0.gguf -b 512 -ngl 99 -p "Introduce yourself"
```

You can now chat with the model.

### 2) Run models without containers (hard way)

This path builds and installs dependencies (which can take some time. For faster builds, allocate more CPUs and memory to the VM. See [`options`](../../reference/limactl_start/#options)). Use Fedora as the image.

{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --vm-type=krunkit template://fedora
limactl shell fedora
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
vmType: krunkit

base:
- template://_images/fedora
- template://_default/mounts

mountType: virtiofs
```
{{% /tab %}}
{{< /tabpane >}}

Once inside the VM, install GPU/Vulkan support:

```bash
sudo install-vulkan-gpu.sh
```

The script will prompt to build and install `llama.cpp` with Venus support from source.

After installation, run:

```bash
llama-cli -m models/Qwen3-1.7B-Q8_0.gguf -b 512 -ngl 99 -p "Introduce yourself"
```

and enjoy chatting with the AI model.

## Notes and caveats
- macOS Ventura or later on Apple Silicon is required.
- To verify GPU/Vulkan in the guest container or VM, use tools like `vulkaninfo --summary`.
- AI models on containers can run on any Linux distribution but without containers Fedora is required.
- For more information about usage of `llama-cli`. See [llama.cpp](https://github.com/ggml-org/llama.cpp) `README.md`.
- Driver architecture details: see [Virtual Machine Drivers](../../dev/drivers).