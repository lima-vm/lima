module github.com/lima-vm/lima

go 1.23.0

require (
	al.essio.dev/pkg/shellescape v1.5.1 // gomodjail:confined
	github.com/AlecAivazis/survey/v2 v2.3.7 // gomodjail:confined
	github.com/Code-Hex/vz/v3 v3.6.0
	github.com/Microsoft/go-winio v0.6.2
	github.com/apparentlymart/go-cidr v1.1.0 // gomodjail:confined
	github.com/balajiv113/fd v0.0.0-20230330094840-143eec500f3e // gomodjail:confined
	github.com/cheggaaa/pb/v3 v3.1.6
	github.com/containerd/containerd v1.7.25 // gomodjail:confined
	github.com/containerd/continuity v0.4.5 // gomodjail:confined
	github.com/containers/gvisor-tap-vsock v0.8.3
	github.com/coreos/go-semver v0.3.1 // gomodjail:confined
	github.com/cpuguy83/go-md2man/v2 v2.0.6 // gomodjail:confined
	github.com/cyphar/filepath-securejoin v0.4.1 // gomodjail:confined
	github.com/digitalocean/go-qemu v0.0.0-20221209210016-f035778c97f7 // gomodjail:confined
	github.com/diskfs/go-diskfs v1.5.0
	github.com/docker/go-units v0.5.0 // gomodjail:confined
	github.com/elastic/go-libaudit/v2 v2.6.1 // gomodjail:confined
	github.com/foxcpp/go-mockdns v1.1.0 // gomodjail:confined
	github.com/goccy/go-yaml v1.15.22 // gomodjail:confined
	github.com/google/go-cmp v0.6.0 // gomodjail:confined
	github.com/google/yamlfmt v0.16.0 // gomodjail:confined
	github.com/invopop/jsonschema v0.13.0 // gomodjail:confined
	github.com/lima-vm/go-qcow2reader v0.6.0 // gomodjail:confined
	github.com/lima-vm/sshocker v0.3.5
	github.com/mattn/go-isatty v0.0.20 // gomodjail:confined
	github.com/mattn/go-shellwords v1.0.12 // gomodjail:confined
	github.com/mdlayher/vsock v1.2.1
	github.com/miekg/dns v1.1.63
	github.com/mikefarah/yq/v4 v4.45.1 // gomodjail:confined
	github.com/nxadm/tail v1.4.11
	github.com/opencontainers/go-digest v1.0.0 // gomodjail:confined
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 // gomodjail:confined
	github.com/rjeczalik/notify v0.9.3 // gomodjail:confined
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.1 // gomodjail:confined
	github.com/sethvargo/go-password v0.3.1 // gomodjail:confined
	github.com/sirupsen/logrus v1.9.4-0.20230606125235-dd1b4c2e81af // gomodjail:confined
	github.com/spf13/cobra v1.8.1
	github.com/spf13/pflag v1.0.6 // gomodjail:confined
	github.com/wk8/go-ordered-map/v2 v2.1.8 // gomodjail:confined
	golang.org/x/net v0.35.0 // gomodjail:confined
	golang.org/x/sync v0.11.0 // gomodjail:confined
	golang.org/x/sys v0.30.0
	golang.org/x/text v0.22.0 // gomodjail:confined
	google.golang.org/grpc v1.70.0 // gomodjail:confined
	google.golang.org/protobuf v1.36.5
	gopkg.in/op/go-logging.v1 v1.0.0-20160211212156-b2cb9fa56473 // gomodjail:confined
	gotest.tools/v3 v3.5.2 // gomodjail:confined
	k8s.io/api v0.32.2 // gomodjail:confined
	k8s.io/apimachinery v0.32.2 // gomodjail:confined
	k8s.io/client-go v0.32.2 // gomodjail:confined
)

require (
	github.com/Code-Hex/go-infinity-channel v1.0.0 // indirect; indirect // gomodjail:confined
	github.com/VividCortex/ewma v1.2.0 // indirect; indirect // gomodjail:confined
	github.com/a8m/envsubst v1.4.2 // indirect; indirect // gomodjail:confined
	github.com/alecthomas/participle/v2 v2.1.1 // indirect; indirect // gomodjail:confined
	github.com/bahlo/generic-list-go v0.2.0 // indirect; indirect // gomodjail:confined
	github.com/bmatcuk/doublestar/v4 v4.7.1 // indirect; indirect // gomodjail:confined
	github.com/braydonk/yaml v0.9.0 // indirect; indirect // gomodjail:confined
	github.com/buger/jsonparser v1.1.1 // indirect; indirect // gomodjail:confined
	github.com/containerd/errdefs v1.0.0 // indirect; indirect // gomodjail:confined
	github.com/containerd/log v0.1.0 // indirect; indirect // gomodjail:confined
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect; indirect // gomodjail:confined
	github.com/digitalocean/go-libvirt v0.0.0-20220804181439-8648fbde413e // indirect; indirect // gomodjail:confined
	github.com/dimchansky/utfbom v1.1.1 // indirect; indirect // gomodjail:confined
	github.com/djherbis/times v1.6.0 // indirect; indirect // gomodjail:confined
	github.com/elliotchance/orderedmap v1.7.1 // indirect; indirect // gomodjail:confined
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect; indirect // gomodjail:confined
	github.com/fatih/color v1.18.0 // indirect; indirect // gomodjail:confined
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect; indirect // gomodjail:confined
	github.com/go-logr/logr v1.4.2 // indirect; indirect // gomodjail:confined
	github.com/go-openapi/jsonpointer v0.21.0 // indirect; indirect // gomodjail:confined
	github.com/go-openapi/jsonreference v0.20.2 // indirect; indirect // gomodjail:confined
	github.com/go-openapi/swag v0.23.0 // indirect; indirect // gomodjail:confined
	github.com/goccy/go-json v0.10.4 // indirect; indirect // gomodjail:confined
	github.com/gogo/protobuf v1.3.2 // indirect; indirect // gomodjail:confined
	github.com/golang/protobuf v1.5.4 // indirect; indirect // gomodjail:confined
	github.com/google/btree v1.1.3 // indirect; indirect // gomodjail:confined
	github.com/google/gnostic-models v0.6.8 // indirect; indirect // gomodjail:confined
	github.com/google/gofuzz v1.2.0 // indirect; indirect // gomodjail:confined
	github.com/google/gopacket v1.1.19 // indirect; indirect // gomodjail:confined
	github.com/google/uuid v1.6.0 // indirect; indirect // gomodjail:confined
	github.com/inconshreveable/mousetrap v1.1.0 // indirect; indirect // gomodjail:confined
	github.com/insomniacslk/dhcp v0.0.0-20240710054256-ddd8a41251c9 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect; indirect // gomodjail:confined
	github.com/josharian/intern v1.0.0 // indirect; indirect // gomodjail:confined
	github.com/json-iterator/go v1.1.12 // indirect; indirect // gomodjail:confined
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect; indirect // gomodjail:confined
	github.com/kr/fs v0.1.0 // indirect; indirect // gomodjail:confined
	github.com/linuxkit/virtsock v0.0.0-20220523201153-1a23e78aa7a2 // indirect; indirect // gomodjail:confined
	github.com/magiconair/properties v1.8.9 // indirect; indirect // gomodjail:confined
	github.com/mailru/easyjson v0.7.7 // indirect; indirect // gomodjail:confined
	github.com/mattn/go-colorable v0.1.14 // indirect; indirect // gomodjail:confined
	github.com/mattn/go-runewidth v0.0.16 // indirect; indirect // gomodjail:confined
	github.com/mdlayher/socket v0.5.1 // indirect; indirect // gomodjail:confined
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect; indirect // gomodjail:confined
	github.com/mitchellh/mapstructure v1.5.0 // indirect; indirect // gomodjail:confined
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect; indirect // gomodjail:confined
	github.com/modern-go/reflect2 v1.0.2 // indirect; indirect // gomodjail:confined
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect; indirect // gomodjail:confined
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect; indirect // gomodjail:confined
	github.com/pierrec/lz4/v4 v4.1.17 // indirect; indirect // gomodjail:confined
	github.com/pkg/errors v0.9.1 // indirect; indirect // gomodjail:confined
	github.com/pkg/sftp v1.13.7 // indirect; indirect // gomodjail:confined
	github.com/rivo/uniseg v0.2.0 // indirect; indirect // gomodjail:confined
	github.com/russross/blackfriday/v2 v2.1.0 // indirect; indirect // gomodjail:confined
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06 // indirect; indirect // gomodjail:confined
	github.com/u-root/uio v0.0.0-20240224005618-d2acac8f3701 // indirect
	github.com/x448/float16 v0.8.4 // indirect; indirect // gomodjail:confined
	github.com/yuin/gopher-lua v1.1.1 // indirect; indirect // gomodjail:confined
	golang.org/x/crypto v0.33.0 // indirect; indirect // gomodjail:confined
	golang.org/x/mod v0.22.0 // indirect; indirect // gomodjail:confined
	golang.org/x/oauth2 v0.24.0 // indirect; indirect // gomodjail:confined
	golang.org/x/term v0.29.0 // indirect; indirect // gomodjail:confined
	golang.org/x/time v0.7.0 // indirect; indirect // gomodjail:confined
	golang.org/x/tools v0.28.0 // indirect; indirect // gomodjail:confined
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241202173237-19429a94021a // indirect; indirect // gomodjail:confined
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect; indirect // gomodjail:confined
	gopkg.in/inf.v0 v0.9.1 // indirect; indirect // gomodjail:confined
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect; indirect // gomodjail:confined
	gopkg.in/yaml.v3 v3.0.1 // indirect; indirect // gomodjail:confined
	gvisor.dev/gvisor v0.0.0-20240916094835-a174eb65023f // indirect
	k8s.io/klog/v2 v2.130.1 // indirect; indirect // gomodjail:confined
	k8s.io/kube-openapi v0.0.0-20241105132330-32ad38e42d3f // indirect; indirect // gomodjail:confined
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738 // indirect; indirect // gomodjail:confined
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect; indirect // gomodjail:confined
	sigs.k8s.io/structured-merge-diff/v4 v4.5.0 // indirect; indirect // gomodjail:confined
	sigs.k8s.io/yaml v1.4.0 // indirect; indirect // gomodjail:confined
)
