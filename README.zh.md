This is an *informal* translation of [`README.md` (revision c1368f45, 2022-Dec-12)](https://github.com/lima-vm/lima/blob/c1368f45d908947dd0828bc5caa00baa4a46be5c/README.md) in Chinese.
This translation might be out of sync with the English version.
Please refer to the [English `README.md`](README.md) for the latest information.

这是 [`README.md` (修订版 c1368f45, 2022-12-12)](https://github.com/lima-vm/lima/blob/c1368f45d908947dd0828bc5caa00baa4a46be5c/README.md) 的*非正式*中文翻译，与英文版相比可能有所延迟。
最新情况请查看[英文版 `README.md`](README.md)。

[[📖**开始使用**]](#开始使用)
[[❓**FAQs & 疑难解答**]](#faqs--疑难解答)

![Lima logo](./docs/images/lima-logo-01.svg)

# Lima: Linux virtual machines (多数情况下在 macOS 上)

Lima 启动了具有自动文件共享和端口转发功能的 Linux 虚拟机（类似于 WSL2），以及 [containerd](https://containerd.io)。

Lima 可以被认为是某种非官方的 "Mac 上的 containerd"。

Lima 预期是在 macOS 宿主上使用，但它在 Linux 宿主上也运行良好。


✅ 自动文件共享

✅ 自动端口转发

✅ 对 [containerd](https://containerd.io) 的内建支持 ([其他的容器引擎也可以使用](./examples))

✅ Intel 宿主上的 Intel 虚拟机

✅ [Intel 宿主上的 ARM 虚拟机](./docs/multi-arch.md)

✅ ARM 宿主上的 ARM 虚拟机

✅ [ARM 宿主上的 Intel 虚拟机](./docs/multi-arch.md)

✅ 各种虚拟机 Linux 发行版：[AlmaLinux](./examples/almalinux.yaml)，[Alpine](./examples/alpine.yaml)，[Arch Linux](./examples/archlinux.yaml)，[Debian](./examples/debian.yaml)，[Fedora](./examples/fedora.yaml)，[openSUSE](./examples/opensuse.yaml)，[Oracle Linux](./examples/oraclelinux.yaml)，[Rocky](./examples/rocky.yaml)，[Ubuntu](./examples/ubuntu.yaml) (默认)，……

相关项目：[sshocker (带有文件共享和端口转发的 ssh)](https://github.com/lima-vm/sshocker)

这个项目与 [The Lima driver project (driver for ARM Mali GPUs)](https://gitlab.freedesktop.org/lima) 无关。

[Talks](docs/talks.md) 页面包含 Lima 相关会议演讲的幻灯片和视频的链接。

## 动机

Lima 的目标是向 Mac 用户推广 [containerd](https://containerd.io) （包括 [nerdctl (contaiNERD ctl)](https://github.com/containerd/nerdctl)），但 Lima 也可以用于非容器应用。

## 社区
### 相关项目

容器环境：
- [Rancher Desktop](https://rancherdesktop.io/): 在桌面上进行 Kubernetes 和容器的管理
- [Colima](https://github.com/abiosoft/colima): 用最小化的安装来在 Mac 上使用 Docker (和 Kubernetes)
- [Finch](https://github.com/runfinch/finch): Finch 是一个用于本地容器开发的命令行客户端

GUI:
- [Lima xbar 插件](https://github.com/unixorn/lima-xbar-plugin): [xbar](https://xbarapp.com/) 插件用于从菜单栏启动/停止虚拟机并查看它们的运行状态。
- [lima-gui](https://github.com/afbjorklund/lima-gui): Lima 的 Qt GUI

### 交流渠道
- [GitHub Discussions](https://github.com/lima-vm/lima/discussions)
- CNCF Slack 上的 `#lima` 频道
  - 新用户：https://slack.cncf.io/
  - 登录：https://cloud-native.slack.com/

### 行为准则
Lima 遵循 [CNCF 行为准则](https://github.com/cncf/foundation/blob/master/code-of-conduct.md)。

## 例子

### uname
```console
$ uname -a
Darwin macbook.local 20.4.0 Darwin Kernel Version 20.4.0: Thu Apr 22 21:46:47 PDT 2021; root:xnu-7195.101.2~1/RELEASE_X86_64 x86_64

$ lima uname -a
Linux lima-default 5.11.0-16-generic #17-Ubuntu SMP Wed Apr 14 20:12:43 UTC 2021 x86_64 x86_64 x86_64 GNU/Linux

$ LIMA_INSTANCE=arm lima uname -a
Linux lima-arm 5.11.0-16-generic #17-Ubuntu SMP Wed Apr 14 20:10:16 UTC 2021 aarch64 aarch64 aarch64 GNU/Linux
```

请查看 [`./docs/multi-arch.md`](./docs/multi-arch.md)，获取 ARM 宿主上的 Intel 虚拟机 和 Intel 宿主上的 ARM 虚拟机 的执行情况。

### 在 macOS 和 Linux 之间共享文件
```console
$ echo "files under /Users on macOS filesystem are readable from Linux" > some-file

$ lima cat some-file
files under /Users on macOS filesystem are readable from Linux

$ lima sh -c 'echo "/tmp/lima is writable from both macOS and Linux" > /tmp/lima/another-file'

$ cat /tmp/lima/another-file
/tmp/lima is writable from both macOS and Linux
```

### 运行 containerd 容器 (与 Docker 容器兼容)
```console
$ lima nerdctl run -d --name nginx -p 127.0.0.1:8080:80 nginx:alpine
```

> 你不用每次都运行 "lima nerdctl"，相反，你可以使用特殊的快捷方式 "nerdctl.lima" 来做同样的事情。默认情况下，它将和 Lima 一起安装，所以，你不需要做任何额外的事情。会有一个名为 nerdctl 的符号链接指向 nerdctl.lima。但这只在目录中没有 nerdctl 条目时才会创建。值得一提的是，它只能通过 make install 创建。不包括在 Homebrew/MacPorts/nix 软件包中。

在 macOS 和 Linux 都可以通过 http://127.0.0.1:8080 访问。

关于如何使用 containerd 和 nerdctl（contaiNERD ctl），请访问 https://github.com/containerd/containerd 和 https://github.com/containerd/nerdctl。

## 开始使用
### 安装

可以直接使用 [Homebrew 上的包](https://github.com/Homebrew/homebrew-core/blob/master/Formula/lima.rb) 进行安装。

```console
brew install lima
```

<details>
<summary>手动安装的步骤</summary>
<p>

#### 安装 QEMU

安装 QEMU 7.0 或更新的版本。

#### 安装 Lima

- 从 https://github.com/lima-vm/lima/releases 下载 Lima 的二进制文件，
  并将其解压到 `/usr/local`（或其他地方）。比如：

```bash
brew install jq
VERSION=$(curl -fsSL https://api.github.com/repos/lima-vm/lima/releases/latest | jq -r .tag_name)
curl -fsSL "https://github.com/lima-vm/lima/releases/download/${VERSION}/lima-${VERSION:1}-$(uname -s)-$(uname -m).tar.gz" | tar Cxzvm /usr/local
```

- 如果想从源码安装 Lima，可以运行 `make && make install`。

> **注意**
> Lima 没有定期在 ARM Mac 进行测试（因为缺乏 CI）。

</p>
</details>

### 用法

```console
[macOS]$ limactl start
...
INFO[0029] READY. Run `lima` to open the shell.

[macOS]$ lima uname
Linux
```

### 命令含义

#### `limactl start`
`limactl start [--name=NAME] [--tty=false] <template://TEMPLATE>`: 启动 Linux 实例

```console
$ limactl start
? Creating an instance "default"  [Use arrows to move, type to filter]
> Proceed with the current configuration
  Open an editor to review or modify the current configuration
  Choose another example (docker, podman, archlinux, fedora, ...)
  Exit
...
INFO[0029] READY. Run `lima` to open the shell.
```

选择 `Proceed with the current configuration`，然后等待宿主终端上显示 "READY" 。

如果想做自动化，`--tty=false` flag 可以禁用用户交互。

##### 高级用法
从 "docker" 模板创建一个 "default" 实例：
```console
$ limactl start --name=default template://docker
```

> 注意：`limactl start template://TEMPLATE` 需要 Lima v0.9.0 或更新版本。
> 老版本应该用 `limactl start /usr/local/share/doc/lima/examples/TEMPLATE.yaml` 替代。

查看模板列表：
```console
$ limactl start --list-templates
```

从本地文件创建 "default" 实例：
```console
$ limactl start --name=default /usr/local/share/lima/examples/fedora.yaml
```

从远程 URL（小心使用，一定要确保来源是可信的）创建 "default" 实例：
```console
$ limactl start --name=default https://raw.githubusercontent.com/lima-vm/lima/master/examples/alpine.yaml
```

#### `limactl shell`
`limactl shell <INSTANCE> <COMMAND>`: 在 Linux 上执行 `<COMMAND>`。

对于 "default" 实例，这条命令可以缩写为 `lima <COMMAND>`。
`lima` 命令也接受环境变量 `$LIMA_INSTANCE` 作为实例名。

#### `limactl copy`
`limactl copy <SOURCE> ... <TARGET>`: 在实例之间，或者宿主与实例之间复制文件

使用 `<INSTANCE>:<FILENAME>` 指定一个实例内的源文件或者目标文件。

#### `limactl list`
`limactl list [--json]`: 列出实例

#### `limactl stop`
`limactl stop [--force] <INSTANCE>`: 停止实例

#### `limactl delete`
`limactl delete [--force] <INSTANCE>`: 删除实例

#### `limactl factory-reset`
`limactl factory-reset <INSTANCE>`: 将实例恢复为初始设置

#### `limactl edit`
`limactl edit <INSTANCE>`: 编辑实例

#### `limactl disk`

`limactl disk create <DISK> --size <SIZE>`: 创建一个要附加到某个实例的外部磁盘

`limactl disk delete <DISK>`: 删除一个已有的磁盘

`limactl disk list`: 列出所有已有的磁盘

#### `limactl completion`
- 要启用 bash 中的自动补全，添加 `source <(limactl completion bash)` 到 `~/.bash_profile` 内。

- 要启用 zsh 中的自动补全，请查看 `limactl completion zsh --help`

### :warning: 警告：确保做好数据备份
Lima 可能存在导致数据丢失的 bug。

**确保在运行 Lima 前做好数据备份。**

尤其需要注意的是，以下数据可能很容易丢失：
- 共享可写目录下的数据（默认路径`/tmp/lima`），
  可能在宿主休眠之后发生（比如，在关闭和重新打开笔记本电脑的盖子之后）
- 虚拟机镜像中的数据，绝大部分发生在升级 Lima 的版本时

### 配置

请参考 [`./examples/default.yaml`](./examples/default.yaml)。

当前虚拟机默认配置：
- OS: Ubuntu 22.10 (Kinetic Kudu)
- CPU: 4 cores
- 内存：4 GiB
- 硬盘：100 GiB
- 挂载目录：`~`（只读）, `/tmp/lima`（可写）
- SSH: 127.0.0.1:60022

## 它是怎么工作的？

- （系统）管理程序：[QEMU 附带 HVF 加速（默认），或者 Virtualization.framework](./docs/vmtype.md)
- 文件共享：[Reverse SSHFS (默认)，或者 virtio-9p-pci 即 virtfs，或者 virtiofs](./docs/mount.md)
- 端口转发：`ssh -L`，通过监视虚拟机的 `/proc/net/tcp` 和 `iptables` 事件来自动化

## 开发者指南

### 给 Lima 做贡献
- 请通过 `git commit -s` 来用你的真实姓名签名你的提交，
  以此确认你的 [Developer Certificate of Origin (DCO)](https://developercertificate.org/)。
- 请合并提交。

### 帮助我们
:pray:
- 文档
- CLI 用户体验
- 性能优化
- Windows 宿主
- 使用 [vsock](https://github.com/apple/darwin-xnu/blob/xnu-7195.81.3/bsd/man/man4/vsock.4) 替换 SSH（这份工作需要在 QEMU repo 内完成）

## FAQs & 疑难解答
<!-- doctoc: https://github.com/thlorenz/doctoc -->

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
### Generic

- [普遍问题](#%E6%99%AE%E9%81%8D%E9%97%AE%E9%A2%98)
  - ["我的登录密码是什么？"](#%E6%88%91%E7%9A%84%E7%99%BB%E5%BD%95%E5%AF%86%E7%A0%81%E6%98%AF%E4%BB%80%E4%B9%88)
  - ["Lima 能在 ARM Mac 上运行吗？"](#lima-%E8%83%BD%E5%9C%A8-arm-mac-%E4%B8%8A%E8%BF%90%E8%A1%8C%E5%90%97)
  - ["我能运行非 Ubuntu 虚拟机吗"](#%E6%88%91%E8%83%BD%E8%BF%90%E8%A1%8C%E9%9D%9E-ubuntu-%E8%99%9A%E6%8B%9F%E6%9C%BA%E5%90%97)
  - ["我能运行其他容器引擎，比如 Docker 和 Podman 吗？Kubernetes 呢？"](#%E6%88%91%E8%83%BD%E8%BF%90%E8%A1%8C%E5%85%B6%E4%BB%96%E5%AE%B9%E5%99%A8%E5%BC%95%E6%93%8E%E6%AF%94%E5%A6%82-docker-%E5%92%8C-podman-%E5%90%97kubernetes-%E5%91%A2)
  - ["我能在远程 Linux 计算机上运行 Lima 吗？"](#%E6%88%91%E8%83%BD%E5%9C%A8%E8%BF%9C%E7%A8%8B-linux-%E8%AE%A1%E7%AE%97%E6%9C%BA%E4%B8%8A%E8%BF%90%E8%A1%8C-lima-%E5%90%97)
  - ["与 Docker for Mac 相比有什么优点？"](#%E4%B8%8E-docker-for-mac-%E7%9B%B8%E6%AF%94%E6%9C%89%E4%BB%80%E4%B9%88%E4%BC%98%E7%82%B9)
- [QEMU](#qemu)
  - ["QEMU 崩溃，提示 `HV_ERROR`"](#qemu-%E5%B4%A9%E6%BA%83%E6%8F%90%E7%A4%BA-hv_error)
  - ["QEMU 很慢"](#qemu-%E5%BE%88%E6%85%A2)
  - [错误 "killed -9"](#%E9%94%99%E8%AF%AF-killed--9)
  - ["QEMU 崩溃，提示 `vmx_write_mem: mmu_gva_to_gpa XXXXXXXXXXXXXXXX failed`"](#qemu-%E5%B4%A9%E6%BA%83%E6%8F%90%E7%A4%BA-vmx_write_mem-mmu_gva_to_gpa-xxxxxxxxxxxxxxxx-failed)
- [网络](#%E7%BD%91%E7%BB%9C)
  - ["从宿主无法访问虚拟机 IP 192.168.5.15"](#%E4%BB%8E%E5%AE%BF%E4%B8%BB%E6%97%A0%E6%B3%95%E8%AE%BF%E9%97%AE%E8%99%9A%E6%8B%9F%E6%9C%BA-ip-192168515)
  - ["Ping 显示重复的数据包和大量的响应时间"](#ping-%E6%98%BE%E7%A4%BA%E9%87%8D%E5%A4%8D%E7%9A%84%E6%95%B0%E6%8D%AE%E5%8C%85%E5%92%8C%E5%A4%A7%E9%87%8F%E7%9A%84%E5%93%8D%E5%BA%94%E6%97%B6%E9%97%B4)
- [文件系统共享](#%E6%96%87%E4%BB%B6%E7%B3%BB%E7%BB%9F%E5%85%B1%E4%BA%AB)
  - ["文件系统很慢"](#%E6%96%87%E4%BB%B6%E7%B3%BB%E7%BB%9F%E5%BE%88%E6%85%A2)
  - ["文件系统不可写"](#%E6%96%87%E4%BB%B6%E7%B3%BB%E7%BB%9F%E4%B8%8D%E5%8F%AF%E5%86%99)
- [外部项目](#%E5%A4%96%E9%83%A8%E9%A1%B9%E7%9B%AE)
  - ["我在使用 Rancher Desktop。怎么处理底层的 Lima？"](#%E6%88%91%E5%9C%A8%E4%BD%BF%E7%94%A8-rancher-desktop-%E6%80%8E%E4%B9%88%E5%A4%84%E7%90%86%E5%BA%95%E5%B1%82%E7%9A%84-lima)
- ["调试其他问题还有什么提示吗？"](#%E8%B0%83%E8%AF%95%E5%85%B6%E4%BB%96%E9%97%AE%E9%A2%98%E8%BF%98%E6%9C%89%E4%BB%80%E4%B9%88%E6%8F%90%E7%A4%BA%E5%90%97)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->
### 普遍问题
#### "我的登录密码是什么？"
默认情况下，密码是被禁用和锁定的。
你应该执行 `limactl shell bash`（或者 `lima bash`）来打开 shell。

还有一种方法，你可以直接 ssh 进虚拟机：`ssh -p 60022 -i ~/.lima/_config/user -o NoHostAuthenticationForLocalhost=yes 127.0.0.1`。

#### "Lima 能在 ARM Mac 上运行吗？"
可以的。不过我们没有定期在 ARM 上进行测试（因为缺乏 CI）。

#### "我能运行非 Ubuntu 虚拟机吗"
AlmaLinux，Alpine，Arch Linux，Debian，Fedora，openSUSE，Oracle Linux，和 Rocky 都是可以运行的。
请查看 [`./examples/`](./examples/) 。

一个镜像必须满足下面的需求：
- systemd 或者 OpenRC
- cloud-init
- 下面的二进制包应该被预装：
  - `sudo`
- 下面的二进制包应该被预装，或者可以通过包管理器安装：
  - `sshfs`
  - `newuidmap` 和 `newgidmap`
- `apt-get`, `dnf`, `apk`, `pacman`, 或者 `zypper` （如果你想贡献对其他包管理器的支持，请执行 `git grep apt-get` 来确定哪里需要改动）

#### "我能运行其他容器引擎，比如 Docker 和 Podman 吗？Kubernetes 呢？"
是的，任何容器引擎都可以和 Lima 配合使用。

容器运行时例子：
- [`./examples/docker.yaml`](./examples/docker.yaml): Docker
- [`./examples/podman.yaml`](./examples/podman.yaml): Podman
- [`./examples/apptainer.yaml`](./examples/apptainer.yaml): Apptainer

容器镜像构建器例子：
- [`./examples/buildkit.yaml`](./examples/buildkit.yaml): BuildKit

容器业务流程协调程序例子：
- [`./examples/k3s.yaml`](./examples/k3s.yaml): Kubernetes (k3s)
- [`./examples/k8s.yaml`](./examples/k8s.yaml): Kubernetes (kubeadm)
- [`./examples/nomad.yaml`](./examples/nomad.yaml): Nomad

默认的 Ubuntu 镜像也包含了 LXD。运行 `lima sudo lxc init` 来设置 LXD。

也可以看看第三方基于 Lima 的 containerd 项目：
- [Rancher Desktop](https://rancherdesktop.io/): 在桌面上进行 Kubernetes 和容器的管理
- [Colima](https://github.com/abiosoft/colima): 用最小化的安装来在 Mac 上使用 Docker (和 Kubernetes)

#### "我能在远程 Linux 计算机上运行 Lima 吗？"
Lima 本身不支持连接到远程 Linux 计算机，但是 Lima 的前身 [sshocker](https://github.com/lima-vm/sshocker) 为远程 Linux 计算机提供了类似的功能。

例如，运行 `sshocker -v /Users/foo:/home/foo/mnt -p 8080:80 <USER>@<HOST>` 将 `/Users/foo` 作为 `/home/foo/mnt` 向远程计算机公开，并将 `localhost:8080` 转发到远程计算机的 80 端口。

#### "与 Docker for Mac 相比有什么优点？"
Lima 是免费软件（Apache License 2.0），但 Docker for Mac 不是。
他们的 [EULA](https://www.docker.com/legal/docker-software-end-user-license-agreement) 甚至禁止披露 benchmark 的结果。

另一方面来说，[Moby](https://github.com/moby/moby)，即 Docker for Linux，也是免费软件，但 Moby/Docker 没有 containerd 的几个新特性，比如：
- [按需拉取镜像（即 lazy-pulling, eStargz）](https://github.com/containerd/nerdctl/blob/master/docs/stargz.md)
- [运行加密容器](https://github.com/containerd/nerdctl/blob/master/docs/ocicrypt.md)
- 导入和导出 [本地 OCI 存档](https://github.com/opencontainers/image-spec/blob/master/image-layout.md)

### QEMU
#### "QEMU 崩溃，提示 `HV_ERROR`"
如果你在 macOS 上通过 homebrew 安装了 QEMU v6.0.0 或更新的版本，你的 QEMU 二进制文件应该已经自动签名以启用 HVF 加速。

但是，如果你看到 `HV_ERROR`，你可能需要对二进制文件进行手动签名。

```bash
cat >entitlements.xml <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>com.apple.security.hypervisor</key>
    <true/>
</dict>
</plist>
EOF

codesign -s - --entitlements entitlements.xml --force /usr/local/bin/qemu-system-x86_64
```

注意：**只有** 10.15.7 **之前**版本的 macOS 上你才可能需要额外添加这个授权：

```
    <key>com.apple.vm.hypervisor</key>
    <true/>
```

#### "QEMU 很慢"
- 确保 HVF 已经通过 `com.apple.security.hypervisor` 授权进行启用。参见 ["QEMU 崩溃，提示 `HV_ERROR`"](#-qemu-崩溃提示-hverror-)
- 模拟非原生计算机（Intel 宿主上的 ARM 虚拟机，ARM 宿主上的 Intel 虚拟机）在设计上就很慢。查看 [`docs/multi-arch.md`](./docs/multi-arch.md) 了解解决方法。

#### 错误 "killed -9"
- 确保 QEMU 已经签名过。参见 ["QEMU 崩溃，提示 `HV_ERROR`"](#-qemu-崩溃提示-hverror-)。
- 如果你是在 macOS 10.15.7 或者 11.0 或者更新的版本上运行，请确保授权 `com.apple.vm.hypervisor` **没有**被添加。它只在旧版本 macOS 上生效。你可以通过执行 `codesign --remove-signature /usr/local/bin/qemu-system-x86_64` 来清理签名然后[重新开始](#开始使用)

#### "QEMU 崩溃，提示 `vmx_write_mem: mmu_gva_to_gpa XXXXXXXXXXXXXXXX failed`"
已知在 Intel Mac 上运行 RHEL8 兼容发行版（如 Rocky Linux 8.x）的镜像时会发生此错误。
解决方式是设置环境变量：`QEMU_SYSTEM_X86_64="qemu-system-x86_64 -cpu Haswell-v4"`。

https://bugs.launchpad.net/qemu/+bug/1838390

### 网络
#### "从宿主无法访问虚拟机 IP 192.168.5.15"

默认虚拟机 IP 192.168.5.15 对宿主和其他虚拟机来说是不可访问的。

要添加另一个 IP 地址给宿主和其他虚拟机访问的话，请启用 [`socket_vmnet`](https://github.com/lima-vm/socket_vmnet) (从 Lima v0.12 起可用) 
或者 [`vde_vmnet`](https://github.com/lima-vm/vde_vmnet) (已弃用).

请查看 [`./docs/network.md`](./docs/network.md)。

#### "Ping 显示重复的数据包和大量的响应时间"

Lima 使用的是 QEMU 的 SLIRP 网络，它不支持开箱即用 `ping`。

```
$ ping google.com
PING google.com (172.217.165.14): 56 data bytes
64 bytes from 172.217.165.14: seq=0 ttl=42 time=2395159.646 ms
64 bytes from 172.217.165.14: seq=0 ttl=42 time=2396160.798 ms (DUP!)
```

更多细节请查看 [Documentation/Networking](https://wiki.qemu.org/Documentation/Networking#User_Networking_.28SLIRP.29)。

### 文件系统共享
#### "文件系统很慢"
试试 virtiofs。请查看 [`docs/mount.md`](./docs/mount.md)。

#### "文件系统不可写"
默认情况下，home 目录是以只读形式挂载的。
如果想启用可写，请在 YAML 中指定 `writable: true`。

```yaml
mounts:
- location: "~"
  writable: true
```

运行 `limactl edit <INSTANCE>` 来为一个实例打开 YAML 编辑器进行编辑。

### 外部项目
#### "我在使用 Rancher Desktop。怎么处理底层的 Lima？"

在 macOS 宿主上，Rancher Desktop（从 v1.0 开始）以以下配置启动 Lima：

- `$LIMA_HOME` 目录：`$HOME/Library/Application Support/rancher-desktop/lima`
- `limactl` 二进制文件：`/Applications/Rancher Desktop.app/Contents/Resources/resources/darwin/lima/bin/limactl`
- Lima 实例名：`0`

如果想要开启一个 shell，运行下面的命令：

```shell
LIMA_HOME="$HOME/Library/Application Support/rancher-desktop/lima" "/Applications/Rancher Desktop.app/Contents/Resources/resources/darwin/lima/bin/limactl" shell 0
```

在 Linux 宿主上，试试以下命令：
```shell
LIMA_HOME="$HOME/.local/share/rancher-desktop/lima" /opt/rancher-desktop/resources/resources/linux/lima/bin/limactl shell 0
```

如果你已经安装了 Rancher Desktop 作为一个 AppImage 的话：
```shell
LIMA_HOME="$HOME/.local/share/rancher-desktop/lima" "$(ls -d /tmp/.mount_ranche*/opt/rancher-desktop/resources/resources/linux/lima/bin)/limactl" shell 0
```

### "有关于调试问题的其他提示吗？"
- 检查日志：
  - `limactl --debug start`
  - `$HOME/.lima/<INSTANCE>/serial.log`
  - `/var/log/cloud-init-output.log` (虚拟机内)
  - `/var/log/cloud-init.log` (虚拟机内)
- 确保你没有在 YAML 文件内混合使用空格和 tab。

- - -
**我们是一个 [Cloud Native Computing Foundation](https://cncf.io/) 沙盒项目。**

<img src="https://www.cncf.io/wp-content/uploads/2022/07/cncf-color-bg.svg" width=300 />

The Linux Foundation® (TLF) has registered trademarks and uses trademarks. For a list of TLF trademarks, see [Trademark Usage](https://www.linuxfoundation.org/trademark-usage/).
