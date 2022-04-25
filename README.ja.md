This is an *informal* translation of [`README.md` (revision 0aebc304, 2022-Apr-25)](https://github.com/lima-vm/lima/blob/0aebc304f8cdc65375f8bfca7414ced6397a4fcc/README.md) in Japanese.
This translation might be out of sync with the English version.
Please refer to the [English `README.md`](README.md) for the latest information.

[`README.md` (リビジョン 0aebc304, 2022年4月25日)](https://github.com/lima-vm/lima/blob/0aebc304f8cdc65375f8bfca7414ced6397a4fcc/README.md)の *非正式* な日本語訳です。
英語版からの翻訳が遅れていることがあります。
最新の情報については[英語版 `README.md`](README.md)をご覧ください。

- - -

[[📖**始める**]](#始める)
[[❓**FAQとトラブルシューティング]**](#FAQとトラブルシューティング)

![Limaロゴ](./docs/images/lima-logo-01.svg)

# Lima: Linux virtual machines (多くの場合、macOSで)

Limaは自動的なファイル共有とポートフォワード機能つきでLinux仮想マシンと[containerd](https://containerd.io) を起動します(WSL2と同様)。

Limaはある種の非公式な「containerd for Mac」とみなすことができます。

LimaはmacOSホストで使用されることを想定していますが、Linuxホストでも使用できます。

✅ 自動的なファイル共有

✅ 自動的なポートフォワーディング

✅ [containerd](https://containerd.io)の組み込みサポート([他のコンテナエンジンも使えます](./examples))

✅ Intelマシン上でのIntel仮想マシン

✅ [Intelマシン上でのARM仮想マシン](./docs/multi-arch.md)

✅ ARMマシン上でのARM仮想マシン

✅ [ARMマシン上でのIntel仮想マシン](./docs/multi-arch.md)

✅ 様々なゲストLinuxディストリビューション: [AlmaLinux](./examples/almalinux.yaml), [Alpine](./examples/alpine.yaml),[Arch Linux](./examples/archlinux.yaml), [Debian](./examples/debian.yaml),[Fedora](./examples/fedora.yaml), [openSUSE](./examples/opensuse.yaml), [Oracle Linux](./examples/oraclelinux.yaml), [Rocky](./examples/rocky.yaml), [Ubuntu](./examples/ubuntu.yaml) (デフォルト), ...

関連するプロジェクト: [sshocker (ファイル共有とポートフォワードがついたSSH)](https://github.com/lima-vm/sshocker)

このプロジェクトは[Limaドライバプロジェクト (ARM Mali GPUのためのドライバ)](https://gitlab.freedesktop.org/lima)とは無関係です。

## 動機

Limaの目的は、Macユーザに[nerdctl (contaiNERDctl)](https://github.com/containerd/nerdctl)を含め[containerd](https://containerd.io)を広めることです。が、Limaではコンテナでないアプリケーションも使用できます。

## コミュニティ
### 採用者

コンテナ環境:
- [Rancher Desktop](https://rancherdesktop.io/): デスクトップで管理できるKubernetesとコンテナ
- [Colima](https://github.com/abiosoft/colima): macOSで小さく始めるDocker(とKubernetes)


GUI:
- [Lima xbar plugin](https://github.com/unixorn/lima-xbar-plugin): [xbar](https://xbarapp.com/) メニューバーから仮想マシンを開始・終了でき、稼働状態を確認できるプラグイン
- [lima-gui](https://github.com/afbjorklund/lima-gui): LimaのQt GUI

### コミュニケーション方法
- [GitHub Discussions](https://github.com/lima-vm/lima/discussions)
- Rancher Users Slackで`#rancher-desktop`チャンネルと同居している`#lima`チャンネル( LimaはRancherのプロジェクトではありません )
  - 新規アカウント: https://slack.rancher.io/
  - ログイン: https://rancher-users.slack.com/

## 例

### uname
```console
$ uname -a
Darwin macbook.local 20.4.0 Darwin Kernel Version 20.4.0: Thu Apr 22 21:46:47 PDT 2021; root:xnu-7195.101.2~1/RELEASE_X86_64 x86_64

$ lima uname -a
Linux lima-default 5.11.0-16-generic #17-Ubuntu SMP Wed Apr 14 20:12:43 UTC 2021 x86_64 x86_64 x86_64 GNU/Linux

$ LIMA_INSTANCE=arm lima uname -a
Linux lima-arm 5.11.0-16-generic #17-Ubuntu SMP Wed Apr 14 20:10:16 UTC 2021 aarch64 aarch64 aarch64 GNU/Linux
```

Intelマシン上でのARM仮想マシン、ARMマシン上でのIntel仮想マシンについては、[./docs/multi-arch.md](./docs/multi-arch.md)を参照してください。

### macOS と Linux の間でファイルを共有する
```console
$ echo "macOSの/Users以下にあるファイルはLinuxから読めます。" > some-file

$ lima cat some-file
macOSの/Users以下にあるファイルはLinuxから読めます。

$ lima sh -c 'echo "/tmp/limaはmacOSとLinuxの両方から書き込めます。" > /tmp/lima/another-file'

$ cat /tmp/lima/another-file
/tmp/limaはmacOSとLinuxの両方から書き込めます。
```

### containerd のコンテナを動かす ( Doker コンテナ互換 )
```console
$ lima nerdctl run -d --name nginx -p 127.0.0.1:8080:80 nginx:alpine
```

> "lima nerdctl"を毎回実行する必要はありません。代わりに"nerdctl.lima"という名前の特別なショートカットを使って同じことを実行できます。
> デフォルトでは、limaと同時にショートカットがインストールされるので、特に何かをする必要はありません。
> nerdctl.limaを指すnerdctlというシンボリックリンクがあるでしょう。このリンクはディレクトリにnerdctlエントリがない場合に限ってのみ作成されますけれども。
> make installをしてインストールしたときにのみこれは作成されることに言及しておいたほうがよいでしょう。Homebrew/MacPorts/nixパッケージには含まれていません。

http://127.0.0.1:8080 はmacOSとLinuxの両方からアクセス可能です。

containerd とnerdctl (contaiNERD ctl)の使い方については、 https://github.com/containerd/containerd と https://github.com/containerd/nerdctl を参照してください。

## 始める
### インストール

[Homebrewパッケージ](https://github.com/Homebrew/homebrew-core/blob/master/Formula/lima.rb)があります。

```console
brew install lima
```

<details>
<summary>手動でのインストール方法</summary>
<p>

#### QEMU をインストールする

最近のバージョンのQEMUをインストールしてください。

M1のmacOSでは、[Homebrew版のQEMU `6.2.0_1`](https://github.com/Homebrew/homebrew-core/pull/96743) 以降が望ましいです。

もしHomebrewを使っていないなら、最近のLinuxゲストを起動するには以下のコミットを含めてください:
- https://github.com/qemu/qemu/commit/ad99f64f `hvf: arm: Use macros for sysreg shift/masking`
- https://github.com/qemu/qemu/commit/7f6c295c `hvf: arm: Handle unknown ID registers as RES0`

これらのコミットはQEMU 7.0には含まれていますが、 [QEMU 7.0はM1で3 GiB以上のメモリを使うのにmacOS 12.4以降を要する点に注意が必要です](https://github.com/lima-vm/lima/pull/796)。

#### Lima をインストールする

- https://github.com/lima-vm/lima/releases からLimaのバイナリアーカイブをダウンロードしてください。そして、アーカイブを`/usr/local` (かどこか)以下に展開してください。例えば:

```bash
brew install jq
VERSION=$(curl -fsSL https://api.github.com/repos/lima-vm/lima/releases/latest | jq -r .tag_name)
curl -fsSL "https://github.com/lima-vm/lima/releases/download/${VERSION}/lima-${VERSION:1}-$(uname -s)-$(uname -m).tar.gz" | tar Cxzvm /usr/local
```

- Limaをソースからインストールするには、`make && make install`を実行してください。

> **注意**
> LimaはARM Macで動作することを定期的にチェックしていません(CIがないため)。

</p>
</details>

### 使い方

```console
[macOS]$ limactl start
...
INFO[0029] READY. Run `lima` to open the shell.

[macOS]$ lima uname
Linux
```

### コマンドリファレンス

#### `limactl start`
`limactl start [--name=NAME] [--tty=false] <template://TEMPLATE>`: start the Linux instance

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

`Proceed with the current configuration` を選び, ホストのターミナルに "READY" と表示されるまで待ってください。
自動化するには、`--tty=false` フラグで対話的ユーザインターフェースを無効化できます。

##### 応用的な使い方
インスタンス "default" を テンプレート "docker" から作成するには:
```console
$ limactl start --name=default template://docker
```

> 注: `limactl start template://TEMPLATE` は Lima v0.9.0 以降を必要とします。
> 古いリリースでは `limactl start /usr/local/share/doc/lima/examples/TEMPLATE.yaml` が代わりに必要です。

テンプレートの一覧を表示するには:
```console
$ limactl start --list-templates
```

インスタンス "default" を ローカルファイルから作成するには:
```console
$ limactl start --name=default /usr/local/share/lima/examples/fedora.yaml
```

インスタンス "default" を リモートのURLから作成するには (信頼できるソースで慎重に使ってください):
```console
$ limactl start --name=default https://raw.githubusercontent.com/lima-vm/lima/master/examples/alpine.yaml
```

#### `limactl shell`
`limactl shell <INSTANCE> <COMMAND>`: `<COMMAND>` を Linux で実行します。

"default" インスタンスについては, このコマンドは `lima <COMMAND>` に短縮できます。.
`lima` コマンドは、インスタンス名を環境変数 `$LIMA_INSTANCE` としても受け付けます。

#### `limactl copy`
`limactl copy <SOURCE> ... <TARGET>`: ファイルをインスタンス間、もしくはインスタンスとホストとの間でコピーします。

インスタンス内のコピー元もしくはコピー先を指定するには、`<INSTANCE>:<FILENAME>` を使ってください。

#### `limactl list`
`limactl list [--json]`: インスタンス一覧を表示します

#### `limactl stop`
`limactl stop [--force] <INSTANCE>`: インスタンスを停止します

#### `limactl delete`
`limactl delete [--force] <INSTANCE>`: インスタンスを削除します

#### `limactl factory-reset`
`limactl factory-reset <INSTANCE>`: インスタンスを初期化します

#### `limactl edit`
`limactl edit <INSTANCE>`: インスタンスを編集します

#### `limactl completion`
- bash補完を有効にするには、`~/.bash_profile`へ`source <(limactl completion bash)`を追加します。

- zsh補完を有効にするには、`limactl completion zsh --help`を参照してください。

### :warning: 警告: データを確実にバックアップしてください
Limaにはデータの喪失を引き起こすバグが含まれているかもしれません。

**Limaを動かす前にデータを確実にバックアップしてください。**

特に、次のようなデータは簡単に失われます:
- 共有の書き込み可能なディレクトリ内のデータ(デフォルトでは`/tmp/lima`)。おそらくはホストマシンのハイバネーション (例: ノートPCの画面を閉じて再度開けた後) のあと
- 仮想マシンのイメージ内のデータ、特にlimaのバージョンをアップグレードしたとき

### 設定

[`./examples/default.yaml`](./examples/default.yaml)を見てください。

現在のデフォルト構成:
- OS: Ubuntu 22.04 (Jammy Jellyfish)
- CPU: 4 コア
- メモリ: 4 GiB
- ストレージ: 100 GiB
- マウント: `~` (読み取りのみ), `/tmp/lima` (書き込み可能)
- SSH: 127.0.0.1:60022

## 動作する仕組み

- ハイパーバイザ: HVFアクセラレータを搭載したQEMU
- ファイルシステム共有: [リバースsshfs (デフォルト)、もしくは virtio-9p-pci またの名を virtfs](./docs/mount.md)
- ポートフォワーディング: ゲストの`/proc/net/tcp`と`iptables`を自動的に見つつ`ssh -L`

## 開発者ガイド

### Lima に貢献する

- `git commit -s`とあなたの実名でサインをすることによってあなたの [Developer Certificate of Origin (DCO)](https://developercertificate.org/)を証明してください。
- コミットをsquashしてください。

### 助けを求めています
:pray:
- パフォーマンス最適化
- より多くのゲストディストリビューション
- Windows ホスト
- virtio-fs で、virtio-9p-pci またの名を virtfs を置き換える (QEMU側リポジトリで作業をする必要があります)
- SSHを置き換える[vsock](https://github.com/apple/darwin-xnu/blob/xnu-7195.81.3/bsd/man/man4/vsock.4)(QEMU側リポジトリで作業をする必要があります)

## FAQとトラブルシューティング
<!-- doctoc: https://github.com/thlorenz/doctoc -->

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
### Generic

- [一般](#%E4%B8%80%E8%88%AC)
  - ["私のログインパスワードは何ですか？"](#%E7%A7%81%E3%81%AE%E3%83%AD%E3%82%B0%E3%82%A4%E3%83%B3%E3%83%91%E3%82%B9%E3%83%AF%E3%83%BC%E3%83%89%E3%81%AF%E4%BD%95%E3%81%A7%E3%81%99%E3%81%8B)
  - ["Lima はARM Macでも動作しますか？"](#lima-%E3%81%AFarm-mac%E3%81%A7%E3%82%82%E5%8B%95%E4%BD%9C%E3%81%97%E3%81%BE%E3%81%99%E3%81%8B)
  - ["Ubuntu以外のゲストを動かすことはできますか？"](#ubuntu%E4%BB%A5%E5%A4%96%E3%81%AE%E3%82%B2%E3%82%B9%E3%83%88%E3%82%92%E5%8B%95%E3%81%8B%E3%81%99%E3%81%93%E3%81%A8%E3%81%AF%E3%81%A7%E3%81%8D%E3%81%BE%E3%81%99%E3%81%8B)
  - ["DockerやPodmanなどの他のコンテナエンジンを実行できますか？Kubernetesは？"](#docker%E3%82%84podman%E3%81%AA%E3%81%A9%E3%81%AE%E4%BB%96%E3%81%AE%E3%82%B3%E3%83%B3%E3%83%86%E3%83%8A%E3%82%A8%E3%83%B3%E3%82%B8%E3%83%B3%E3%82%92%E5%AE%9F%E8%A1%8C%E3%81%A7%E3%81%8D%E3%81%BE%E3%81%99%E3%81%8Bkubernetes%E3%81%AF)
  - ["LimaをリモートのLinuxマシンで動かせますか？"](#lima%E3%82%92%E3%83%AA%E3%83%A2%E3%83%BC%E3%83%88%E3%81%AElinux%E3%83%9E%E3%82%B7%E3%83%B3%E3%81%A7%E5%8B%95%E3%81%8B%E3%81%9B%E3%81%BE%E3%81%99%E3%81%8B)
  - ["LimaのDocker for Macに対する優位性は？"](#lima%E3%81%AEdocker-for-mac%E3%81%AB%E5%AF%BE%E3%81%99%E3%82%8B%E5%84%AA%E4%BD%8D%E6%80%A7%E3%81%AF)
- [QEMU](#qemu)
  - ["`HV_ERROR`でQEMUがクラッシュします"](#hv_error%E3%81%A7qemu%E3%81%8C%E3%82%AF%E3%83%A9%E3%83%83%E3%82%B7%E3%83%A5%E3%81%97%E3%81%BE%E3%81%99)
  - ["QEMUが遅いです"](#qemu%E3%81%8C%E9%81%85%E3%81%84%E3%81%A7%E3%81%99)
  - ["killed -9" エラー](#killed--9-%E3%82%A8%E3%83%A9%E3%83%BC)
  - ["`vmx_write_mem: mmu_gva_to_gpa XXXXXXXXXXXXXXXX failed`でQEMUがクラッシュします"](#vmx_write_mem-mmu_gva_to_gpa-xxxxxxxxxxxxxxxx-failed%E3%81%A7qemu%E3%81%8C%E3%82%AF%E3%83%A9%E3%83%83%E3%82%B7%E3%83%A5%E3%81%97%E3%81%BE%E3%81%99)
- [SSH](#ssh)
  - ["ポートフォワーディングが動きません"](#%E3%83%9D%E3%83%BC%E3%83%88%E3%83%95%E3%82%A9%E3%83%AF%E3%83%BC%E3%83%87%E3%82%A3%E3%83%B3%E3%82%B0%E3%81%8C%E5%8B%95%E3%81%8D%E3%81%BE%E3%81%9B%E3%82%93)
  - ["Waiting for the essential requirement 1 of X: "ssh" で固まります"](#waiting-for-the-essential-requirement-1-of-x-ssh-%E3%81%A7%E5%9B%BA%E3%81%BE%E3%82%8A%E3%81%BE%E3%81%99)
  - [`limactl cp`コマンドで"Permission denied"](#limactl-cp%E3%82%B3%E3%83%9E%E3%83%B3%E3%83%89%E3%81%A7permission-denied)
- [ネットワーク](#%E3%83%8D%E3%83%83%E3%83%88%E3%83%AF%E3%83%BC%E3%82%AF)
  - ["ホストからゲストのIP 192.168.5.15にアクセスできない"](#%E3%83%9B%E3%82%B9%E3%83%88%E3%81%8B%E3%82%89%E3%82%B2%E3%82%B9%E3%83%88%E3%81%AEip-192168515%E3%81%AB%E3%82%A2%E3%82%AF%E3%82%BB%E3%82%B9%E3%81%A7%E3%81%8D%E3%81%AA%E3%81%84)
  - [Pingのパケットが重複してたり応答が極めて遅かったりする](#ping%E3%81%AE%E3%83%91%E3%82%B1%E3%83%83%E3%83%88%E3%81%8C%E9%87%8D%E8%A4%87%E3%81%97%E3%81%A6%E3%81%9F%E3%82%8A%E5%BF%9C%E7%AD%94%E3%81%8C%E6%A5%B5%E3%82%81%E3%81%A6%E9%81%85%E3%81%8B%E3%81%A3%E3%81%9F%E3%82%8A%E3%81%99%E3%82%8B)
- [外部プロジェクト](#%E5%A4%96%E9%83%A8%E3%83%97%E3%83%AD%E3%82%B8%E3%82%A7%E3%82%AF%E3%83%88)
  - ["Rancher Desktopを使っています。内蔵されているLimaを弄るにはどうすればよいですか。"](#rancher-desktop%E3%82%92%E4%BD%BF%E3%81%A3%E3%81%A6%E3%81%84%E3%81%BE%E3%81%99%E5%86%85%E8%94%B5%E3%81%95%E3%82%8C%E3%81%A6%E3%81%84%E3%82%8Blima%E3%82%92%E5%BC%84%E3%82%8B%E3%81%AB%E3%81%AF%E3%81%A9%E3%81%86%E3%81%99%E3%82%8C%E3%81%B0%E3%82%88%E3%81%84%E3%81%A7%E3%81%99%E3%81%8B)
- ["ほかの問題をデバッグするためのヒントは？"](#%E3%81%BB%E3%81%8B%E3%81%AE%E5%95%8F%E9%A1%8C%E3%82%92%E3%83%87%E3%83%90%E3%83%83%E3%82%B0%E3%81%99%E3%82%8B%E3%81%9F%E3%82%81%E3%81%AE%E3%83%92%E3%83%B3%E3%83%88%E3%81%AF)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->
### 一般
#### "私のログインパスワードは何ですか？"
デフォルトではパスワードは無効化されロックされています。シェルを開くためには`limactl shell bash`(または`lima bash`)を使う必要があります。

代わりに、ゲストに直接SSHすることもできます: `ssh -p 60022 -i ~/.lima/_config/user -o NoHostAuthenticationForLocalhost=yes 127.0.0.1`。

#### "Lima はARM Macでも動作しますか？"
はい。動くでしょう。ただ、ARM Macでの定期的なテストはなされていません(CIがないため)。

#### "Ubuntu以外のゲストを動かすことはできますか？"
AlmaLinux, Alpine, Arch Linux, Debian, Fedora, openSUSE, Oracle Linux, Rocky が動くことが知られています。[`./examples/`](./examples/)を見てください。

イメージは次の要件を満たす必要があります。
- systemdまたはOpenRC
- cloud-init
- 次ののバイナリがプリインストールされていること
  - `sudo`
- 次ののバイナリがプリインストールされていること、あるいはパッケージマネージャからインストールできること
  - `sshfs`
  - `newuidmap` と `newgidmap`
- `apt-get`, `dnf`, `apk`, `pacman`, または `zypper`
  (他のパッケージマネージャへの対応をしようと考えている方は、`git grep
  apt-get`を実行してどこを編集すればよいか確認してください)

#### "DockerやPodmanなどの他のコンテナエンジンを実行できますか？Kubernetesは？"
はい。どのコンテナエンジンもLimaで動くはずです。

コンテナランタイムの例:
- [`./examples/docker.yaml`](./examples/docker.yaml): Docker
- [`./examples/podman.yaml`](./examples/podman.yaml): Podman
- [`./examples/singularity.yaml`](./examples/singularity.yaml): Singularity

コンテナイメージビルダの例:
- [`./examples/buildkit.yaml`](./examples/buildkit.yaml): BuildKit

コンテナオーケストレータの例:
- [`./examples/k3s.yaml`](./examples/k3s.yaml): Kubernetes (k3s)
- [`./examples/k8s.yaml`](./examples/k8s.yaml): Kubernetes (kubeadm)
- [`./examples/nomad.yaml`](./examples/nomad.yaml): Nomad

デフォルトのUbuntuイメージはLXDも含んでいます。LXDを設定するには`lima sudo lxc init`を実行します。

Limaをもとにしたサードパーティのcontainerdプロジェクトも確認してください。
- [Rancher Desktop](https://rancherdesktop.io/): デスクトップで管理できるKubernetesとコンテナ
- [Colima](https://github.com/abiosoft/colima): 小さく始めるDocker(とKubernetes)

#### "LimaをリモートのLinuxマシンで動かせますか？"
Lima自体はリモートのLinuxマシンへの接続はサポートしていません。が、Limaの先駆けである[sshocker](https://github.com/lima-vm/sshocker)はリモートのLinuxマシンへの似たような機能を提供します。

例: `sshocker -v /Users/foo:/home/foo/mnt -p 8080:80 <USER>@<HOST>`を実行して`/Users/foo`を`/home/foo/mnt`としてリモートマシンにさらします。そして、`localhost:8080`でリモートマシンの80番ポートへフォワードします。

#### "LimaのDocker for Macに対する優位性は？"
Limaは自由ソフトウェア(フリーソフトウェア) (Apacheライセンス 2.0)ですが、Docker for Macはそうではありません。彼らの[EULA](https://www.docker.com/legal/docker-software-end-user-license-agreement)はベンチマークの結果を公開することすら禁じています。

一方で、 [Moby](https://github.com/moby/moby)、またの名をDocker for Linux、は自由ソフトウェア(フリーソフトウェア)ですが、Moby/Dockerにはcontainerdのいくつかの新機能が欠けています。例えば:
- [オンデマンドのイメージ取得(いわゆるlazy-pulling, eStargz)](https://github.com/containerd/nerdctl/blob/master/docs/stargz.md)
- [暗号化されたコンテナの実行](https://github.com/containerd/nerdctl/blob/master/docs/ocicrypt.md)
- [ローカルOCIアーカイブ](https://github.com/opencontainers/image-spec/blob/master/image-layout.md)のインポートとエクスポート

### QEMU
#### "`HV_ERROR`でQEMUがクラッシュします"
macOS 11のHomebrewでQEMU v6.0.0かそれ以降をインストールしている場合、QEMUバイナリはおそらく既に自動的にHVFアクセラレーションを有効にする署名がなされています。

しかし、`HV_ERROR`が表示される場合、手動でバイナリに署名をする必要があるかもしれません。

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

注意: 10.15.7**より前**のmacOSバージョン**のみ**、追加でこのentitlementを追加する必要があるかもしれません。

```
    <key>com.apple.vm.hypervisor</key>
    <true/>
```

#### "QEMUが遅いです"
- `com.apple.security.hypervisor`entitlementでHVFバイナリが有効になっていることを確認してください。["QEMU crashes with`HV_ERROR`"](#qemu-crashes-with-hv_error)を参照してください。
- ネイティブでないマシン(Intelマシン上でARM仮想マシン、ARMマシン上でIntel仮想マシン)のエミュレーションは設計からして遅いです。ワークアラウンドについては [`docs/multi-arch.md`](./docs/multi-arch.md) を参照してください。

#### "killed -9" エラー
- QEMUバイナリが署名されていることを確認してください。 ["QEMU crashes with `HV_ERROR`"](#qemu-crashes-with-hv_error)を参照してください。
- macOS 10.15.7または11.0またはそれ以降のmacOSを使用している場合、`com.apple.vm.hypervisor`entitlementが**追加されていないこと**を確認してください。このentitlementは古いmacOSのバージョンでのみ動作します。`codesign --remove-signature /usr/local/bin/qemu-system-x86_64`で署名をクリアできます。そして、[最初からやり直してください](#getting-started)

#### "`vmx_write_mem: mmu_gva_to_gpa XXXXXXXXXXXXXXXX failed`でQEMUがクラッシュします"
このエラーはRocky Linux 8.x などのようなRHEL8互換ディストリビューションのイメージをIntel Macで動かしたときに発生することが知られています。ワークアラウンドは、`$QEMU_SYSTEM_X86_64="qemu-system-x86_64 -cpu Haswell-v4`環境変数をセットすることです。

https://bugs.launchpad.net/qemu/+bug/1838390

### SSH
#### "ポートフォワーディングが動きません"
Lima v0.7.0以前では、Limaは特権ポート(1-1023)のフォワーディングはサポートしていませんでした。例: 80番ではなく8080番を使わなければなりませんでした。

macOSホストのLima v0.7.0 とそれ以降のバージョンでは、特権ポートのフォワーディングをサポートしています。

Linuxホストでは、sysctlの値`net.ipv4.ip_unprivileged_port_start=0`をセットする必要があるかもしれません。

#### "Waiting for the essential requirement 1 of X: "ssh" で固まります"


M1のmacOSでは、最近のLinuxゲストを実行するには[Homebrew版のQEMU `6.2.0_1`](https://github.com/Homebrew/homebrew-core/pull/96743) 以降が必要です。
`brew upgrade` を実行してQEMUを更新してください。

もしHomebrewを使っていないならば、[インストール](#インストール)の節の「手動でのインストール方法」をご覧ください。

デバッグするには、`~/.lima/<インスタンス>` にある `serial.log` もご覧ください。

#### `limactl cp`コマンドで"Permission denied"

`copy`コマンドはLima 0.5.0かそれ以降で作成されたインスタンスでのみ動作します。`INSTANCE`を実際のインスタンス名に置き換えることで古いインスタンスでの手動で必要なアイデンティティをインストールすることができます。

```console
< ~/.lima/_config/user.pub limactl shell INSTANCE sh -c 'tee -a ~/.ssh/authorized_keys'
```

### ネットワーク
#### "ホストからゲストのIP 192.168.5.15にアクセスできない"

デフォルトのIP 192.168.5.15 はホストや他のゲストからアクセスできません。

ホストや他の仮想マシンからアクセス可能な別のIPアドレスを追加するためには、[`vde_vmnet`](https://github.com/lima-vm/vde_vmnet)を有効にしてください。

[`./docs/network.md`](./docs/network.md)を参照してください。

#### Pingのパケットが重複してたり応答が極めて遅かったりする

LimaはQEMUのSLIRPネットワークを使うので`ping`はそのままでは動きません:

```
$ ping google.com
PING google.com (172.217.165.14): 56 data bytes
64 bytes from 172.217.165.14: seq=0 ttl=42 time=2395159.646 ms
64 bytes from 172.217.165.14: seq=0 ttl=42 time=2396160.798 ms (DUP!)
```

詳しくは, [Documentation/Networking](https://wiki.qemu.org/Documentation/Networking#User_Networking_.28SLIRP.29)をご覧ください。

### 外部プロジェクト
#### "Rancher Desktopを使っています。内蔵されているLimaを弄るにはどうすればよいですか。"

macOSホストでは, Rancher Desktop (v1.0現在)はLimaを以下の設定で起動します:

- `$LIMA_HOME` ディレクトリ: `$HOME/Library/Application Support/rancher-desktop/lima`
- `limactl` バイナリ: `/Applications/Rancher Desktop.app/Contents/Resources/resources/darwin/lima/bin/limactl`
- Lima インスタンス名: `0`

シェルを開くには、次のコマンドを実行します:

```shell
LIMA_HOME="$HOME/Library/Application Support/rancher-desktop/lima" "/Applications/Rancher Desktop.app/Contents/Resources/resources/darwin/lima/bin/limactl" shell 0
```

Linuxホストでは次のコマンドを試してみてください:
```shell
LIMA_HOME="$HOME/.local/share/rancher-desktop/lima" /opt/rancher-desktop/resources/resources/linux/lima/bin/limactl shell 0
```

Rancher DesktopをAppImageとしてインストールした場合は:
```shell
LIMA_HOME="$HOME/.local/share/rancher-desktop/lima" "$(ls -d /tmp/.mount_ranche*/opt/rancher-desktop/resources/resources/linux/lima/bin)/limactl" shell 0
```

### "ほかの問題をデバッグするためのヒントは？"
- ログを調査する:
  - `limactl --debug start`
  - `$HOME/.lima/<INSTANCE>/serial.log`
  - `/var/log/cloud-init-output.log` (ゲスト内)
  - `/var/log/cloud-init.log` (ゲスト内)
- YAML内にタブとスペースを混在させていないことを確認してください。
