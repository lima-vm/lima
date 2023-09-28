This is an *informal* translation of [`README.md` (revision 6938ae5f, 2023-Sep-29)](https://github.com/lima-vm/lima/blob/6938ae5fc8eaf1dec9a99011f775e571a37601ec/README.md) in Japanese.
This translation might be out of sync with the English version.
Please refer to the [English `README.md`](README.md) for the latest information.

[`README.md` (リビジョン 6938ae5f, 2023年09月29日)](https://github.com/lima-vm/lima/blob/6938ae5fc8eaf1dec9a99011f775e571a37601ec/README.md)の *非正式* な日本語訳です。
英語版からの翻訳が遅れていることがあります。
最新の情報については[英語版 `README.md`](README.md)をご覧ください。

- - -

[[🌎**ウェブサイト**]](https://lima-vm.io/)
[[📖**ドキュメント**]](https://lima-vm.io/docs/)
[[👤**Slack (`#lima`)**]](https://slack.cncf.io)

<img src="https://lima-vm.io/images/logo.svg" width=400 />

# Lima: Linux Machines

[Lima](https://lima-vm.io/)は自動的なファイル共有とポートフォワード機能つきでLinux仮想マシンを起動します(WSL2と同様)。

Limaは、Macユーザへ[nerdctl (contaiNERD ctl)](https://github.com/containerd/nerdctl)を含む[containerd](https://containerd.io)を普及させることを当初の最終目標に据えていました。しかし、Limaではコンテナ化されていないアプリケーションも実行することができます。

Limaは他のコンテナエンジン(Docker, Podman, Kubernetes 等)やmacOS以外のホスト(Linux, NetBSD 等)での動作もサポートしています。

## はじめの一歩

セットアップ (macOSにて):
```bash
brew install lima
limactl start
```

Linuxコマンドを実行するには:
```bash
lima sudo apt-get install -y neofetch
lima neofetch
```

containerdを用いてコンテナを実行するには:
```bash
lima nerdctl run --rm hello-world
```

Dockerを用いてコンテナを実行するには:
```bash
limactl start template://docker
export DOCKER_HOST=$(limactl list docker --format 'unix://{{.Dir}}/sock/docker.sock')
docker run --rm hello-world
```

Kubernetesを用いてコンテナを実行するには:
```bash
limactl start template://k8s
export KUBECONFIG=$(limactl list k8s --format 'unix://{{.Dir}}/copied-from-guest/kubeconfig.yaml')
kubectl apply -f ...
```

詳しくは <https://lima-vm.io/docs/> をご覧ください。

## コミュニティ
<!-- TODO: このセクションの大部分を https://lima-vm.io/community/ に移動するかコピーする -->
### 採用者

コンテナ環境:
- [Rancher Desktop](https://rancherdesktop.io/): デスクトップで管理できるKubernetesとコンテナ
- [Colima](https://github.com/abiosoft/colima): macOSで小さく始めるDocker(とKubernetes)
- [Finch](https://github.com/runfinch/finch): Finchはローカルでのコンテナ開発用のコマンドラインクライアント
- [Podman Desktop](https://podman-desktop.io/): Podman Desktop GUIにはLimaのプラグインが用意されています

GUI:
- [Lima xbar plugin](https://github.com/unixorn/lima-xbar-plugin): [xbar](https://xbarapp.com/) メニューバーから仮想マシンを開始・終了でき、稼働状態を確認できるプラグイン
- [lima-gui](https://github.com/afbjorklund/lima-gui): LimaのQt GUI

### 連絡手段
- [GitHub Discussions](https://github.com/lima-vm/lima/discussions)
- CNCF Slackの`#lima`チャンネル
  - 新規アカウント: <https://slack.cncf.io/>
  - ログイン: <https://cloud-native.slack.com/>

### 行動規範
Limaは[CNCF行動規範](https://github.com/cncf/foundation/blob/master/code-of-conduct.md)に従います。

**私たちは [Cloud Native Computing Foundation](https://cncf.io/) sandbox project です。**

<img src="https://www.cncf.io/wp-content/uploads/2022/07/cncf-color-bg.svg" width=300 />

The Linux Foundation® (TLF) has registered trademarks and uses trademarks. For a list of TLF trademarks, see [Trademark Usage](https://www.linuxfoundation.org/trademark-usage/).
