// gomodjail:confined
module github.com/lima-vm/lima/v2

go 1.26.8

// Our own packages and golang.org/x packages are trusted
//gosocialcheck:trusted
require (
	github.com/lima-vm/go-qcow2reader v0.8.0 // gomodjail:unconfined
	github.com/lima-vm/sshocker v0.3.11 // gomodjail:unconfined
	golang.org/x/image v0.47.0
	golang.org/x/net v0.61.0 // gomodjail:unconfined
	golang.org/x/sync v0.24.0
	golang.org/x/sys v0.49.0 // gomodjail:unconfined
	golang.org/x/text v0.43.0
)

require (
	al.essio.dev/pkg/shellescape v1.6.1
	github.com/AlecAivazis/survey/v2 v2.3.7 // gomodjail:unconfined
	github.com/Code-Hex/vz/v3 v3.8.0 // gomodjail:unconfined
	github.com/Microsoft/go-winio v0.6.4 // gomodjail:unconfined
	github.com/Microsoft/hcsshim v0.15.0
	github.com/apparentlymart/go-cidr v1.1.1
	github.com/balajiv113/fd v0.0.0-20230330094840-143eec500f3e // gomodjail:unconfined
	github.com/cheggaaa/pb/v3 v3.2.1 // gomodjail:unconfined
	github.com/cilium/ebpf v0.22.0 // gomodjail:unconfined
	github.com/containerd/continuity v0.5.0 // gomodjail:unconfined
	github.com/containers/gvisor-tap-vsock v0.9.0 // gomodjail:unconfined
	github.com/coreos/go-semver v0.3.1
	github.com/coreos/go-systemd/v22 v22.7.0 // gomodjail:unconfined
	github.com/cpuguy83/go-md2man/v2 v2.0.7
	github.com/digitalocean/go-qemu v0.0.0-20221209210016-f035778c97f7 // gomodjail:unconfined
	github.com/diskfs/go-diskfs v1.9.4 // gomodjail:unconfined
	github.com/docker/go-units v0.5.0
	github.com/foxcpp/go-mockdns v1.3.0
	github.com/gdamore/tcell/v2 v2.13.10 // gomodjail:unconfined
	github.com/goccy/go-yaml v1.19.2 // gomodjail:unconfined
	github.com/google/go-cmp v0.7.0
	github.com/google/yamlfmt v0.21.0 // gomodjail:unconfined
	github.com/inetaf/tcpproxy v0.0.0-20260515195445-c159a6051109 // gomodjail:unconfined
	github.com/invopop/jsonschema v0.14.0 // gomodjail:unconfined
	github.com/klauspost/compress v1.20.3
	github.com/mattn/go-isatty v0.0.24 // gomodjail:unconfined
	github.com/mattn/go-shellwords v1.0.16 // gomodjail:unconfined
	github.com/mdlayher/netlink v1.11.2 // gomodjail:unconfined
	github.com/mdlayher/vsock v1.3.0 // gomodjail:unconfined
	github.com/miekg/dns v1.1.73 // gomodjail:unconfined
	github.com/mikefarah/yq/v4 v4.54.1 // gomodjail:unconfined
	github.com/modelcontextprotocol/go-sdk v1.8.0 // gomodjail:unconfined
	github.com/nxadm/tail v1.4.11 // gomodjail:unconfined
	github.com/opencontainers/go-digest v1.0.0
	github.com/pb33f/ordered-map/v2 v2.3.2
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 // gomodjail:unconfined
	github.com/pkg/sftp v1.13.11 // gomodjail:unconfined
	github.com/rivo/tview v0.42.0 // gomodjail:unconfined
	github.com/rjeczalik/notify v0.9.3 // gomodjail:unconfined
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // gomodjail:unconfined
	github.com/sethvargo/go-password v0.4.0
	github.com/sirupsen/logrus v1.10.2 // gomodjail:unconfined
	github.com/spakin/netpbm v1.3.2
	github.com/spf13/cobra v1.10.2 // gomodjail:unconfined
	github.com/spf13/pflag v1.0.10
	google.golang.org/grpc v1.84.0 // gomodjail:unconfined
	google.golang.org/protobuf v1.36.12 // gomodjail:unconfined
	gopkg.in/op/go-logging.v1 v1.0.0-20160211212156-b2cb9fa56473 // gomodjail:unconfined
	gotest.tools/v3 v3.5.2
)

//gosocialcheck:trusted
require (
	// gomodjail:unconfined
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/mod v0.41.0 // indirect
	// gomodjail:unconfined
	golang.org/x/oauth2 v0.36.0 // indirect
	// gomodjail:unconfined
	golang.org/x/term v0.47.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	golang.org/x/tools v0.51.0 // indirect
)

require (
	github.com/Code-Hex/go-infinity-channel v1.0.0 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/a8m/envsubst v1.4.3 // indirect
	github.com/agext/levenshtein v1.2.1 // indirect
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	// gomodjail:unconfined
	github.com/bmatcuk/doublestar/v4 v4.7.1 // indirect
	github.com/buger/jsonparser v1.1.2 // indirect
	github.com/containerd/cgroups/v3 v3.1.3 // indirect
	github.com/containerd/log v0.1.0 // indirect
	// gomodjail:unconfined
	github.com/digitalocean/go-libvirt v0.0.0-20220804181439-8648fbde413e // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	// gomodjail:unconfined
	github.com/djherbis/times v1.6.0 // indirect
	github.com/elliotchance/orderedmap v1.8.0 // indirect
	// gomodjail:unconfined
	github.com/fatih/color v1.19.0 // indirect
	// gomodjail:unconfined
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/gdamore/encoding v1.0.1 // indirect
	// gomodjail:unconfined
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/jsonschema-go v0.4.3 // indirect
	github.com/hashicorp/hcl/v2 v2.25.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	// gomodjail:unconfined
	github.com/insomniacslk/dhcp v0.0.0-20240710054256-ddd8a41251c9 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	// gomodjail:unconfined
	github.com/kr/fs v0.1.0 // indirect
	github.com/linuxkit/virtsock v0.0.0-20241009230534-cb6a20cc0422 // indirect
	github.com/lucasb-eyer/go-colorful v1.3.0 // indirect
	// gomodjail:unconfined
	github.com/magiconair/properties v1.18.12 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-runewidth v0.0.27 // indirect
	// gomodjail:unconfined
	github.com/mdlayher/socket v0.6.0 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
	github.com/pierrec/lz4/v4 v4.1.26 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	// gomodjail:unconfined
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06 // indirect
	github.com/segmentio/asm v1.2.1 // indirect
	github.com/segmentio/encoding v0.5.4 // indirect
	// gomodjail:unconfined
	github.com/u-root/uio v0.0.0-20240224005618-d2acac8f3701 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	// gomodjail:unconfined
	github.com/yuin/gopher-lua v1.1.2 // indirect
	github.com/zclconf/go-cty v1.19.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.6 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260928230214-8a89bd6388cc // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	// gomodjail:unconfined
	gvisor.dev/gvisor v0.0.0-20260413194555-9680d69bf798 // indirect
)

require (
	github.com/apparentlymart/go-textseg/v17 v17.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.2.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/pb33f/go-yaml v0.1.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.47.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	golang.org/x/exp v0.0.0-20241108190413-2d47ceb2692f // indirect
)
