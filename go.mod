// gomodjail:confined
module github.com/lima-vm/lima/v2

go 1.24.0

require (
	al.essio.dev/pkg/shellescape v1.6.0
	github.com/AlecAivazis/survey/v2 v2.3.7
	github.com/Code-Hex/vz/v3 v3.7.1 // gomodjail:unconfined
	github.com/Microsoft/go-winio v0.6.2 // gomodjail:unconfined
	github.com/apparentlymart/go-cidr v1.1.0
	github.com/balajiv113/fd v0.0.0-20230330094840-143eec500f3e
	github.com/cheggaaa/pb/v3 v3.1.7 // gomodjail:unconfined
	github.com/cilium/ebpf v0.19.0 // gomodjail:unconfined
	github.com/containerd/continuity v0.4.5
	github.com/containers/gvisor-tap-vsock v0.8.7 // gomodjail:unconfined
	github.com/coreos/go-semver v0.3.1
	github.com/cpuguy83/go-md2man/v2 v2.0.7
	github.com/digitalocean/go-qemu v0.0.0-20221209210016-f035778c97f7
	github.com/diskfs/go-diskfs v1.7.0 // gomodjail:unconfined
	github.com/docker/go-units v0.5.0
	github.com/foxcpp/go-mockdns v1.1.0
	github.com/goccy/go-yaml v1.18.0
	github.com/google/go-cmp v0.7.0
	github.com/google/yamlfmt v0.17.2
	github.com/invopop/jsonschema v0.13.0
	github.com/lima-vm/go-qcow2reader v0.6.0
	github.com/lima-vm/sshocker v0.3.8 // gomodjail:unconfined
	github.com/mattn/go-isatty v0.0.20
	github.com/mattn/go-shellwords v1.0.12
	github.com/mdlayher/netlink v1.8.0
	github.com/mdlayher/vsock v1.2.1 // gomodjail:unconfined
	github.com/miekg/dns v1.1.68 // gomodjail:unconfined
	github.com/mikefarah/yq/v4 v4.47.2
	github.com/modelcontextprotocol/go-sdk v1.0.0
	github.com/nxadm/tail v1.4.11 // gomodjail:unconfined
	github.com/opencontainers/go-digest v1.0.0
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58
	github.com/pkg/sftp v1.13.9
	github.com/rjeczalik/notify v0.9.3
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2
	github.com/sethvargo/go-password v0.3.1
	github.com/sirupsen/logrus v1.9.4-0.20230606125235-dd1b4c2e81af
	github.com/spf13/cobra v1.10.1 // gomodjail:unconfined
	github.com/spf13/pflag v1.0.10
	github.com/wk8/go-ordered-map/v2 v2.1.8
	golang.org/x/net v0.44.0
	golang.org/x/sync v0.17.0
	golang.org/x/sys v0.36.0 // gomodjail:unconfined
	golang.org/x/text v0.29.0
	google.golang.org/grpc v1.75.1
	google.golang.org/protobuf v1.36.10 // gomodjail:unconfined
	gopkg.in/op/go-logging.v1 v1.0.0-20160211212156-b2cb9fa56473
	gotest.tools/v3 v3.5.2
	k8s.io/api v0.34.1
	k8s.io/apimachinery v0.34.1
)

require (
	github.com/Code-Hex/go-infinity-channel v1.0.0 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/a8m/envsubst v1.4.3 // indirect
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/bmatcuk/doublestar/v4 v4.7.1 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/digitalocean/go-libvirt v0.0.0-20220804181439-8648fbde413e // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/djherbis/times v1.6.0 // indirect
	github.com/elliotchance/orderedmap v1.8.0 // indirect
	github.com/fatih/color v1.18.0 // indirect
	// gomodjail:unconfined
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	// gomodjail:unconfined
	github.com/insomniacslk/dhcp v0.0.0-20240710054256-ddd8a41251c9 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/linuxkit/virtsock v0.0.0-20220523201153-1a23e78aa7a2 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06 // indirect
	// gomodjail:unconfined
	github.com/u-root/uio v0.0.0-20240224005618-d2acac8f3701 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	golang.org/x/crypto v0.42.0 // indirect
	golang.org/x/mod v0.27.0 // indirect
	golang.org/x/term v0.35.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	golang.org/x/tools v0.36.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250707201910-8d1bb00bc6a7 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	// gomodjail:unconfined
	gvisor.dev/gvisor v0.0.0-20240916094835-a174eb65023f // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20250604170112-4c0f3b243397 // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
)

require (
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/google/jsonschema-go v0.3.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0 // indirect
)
