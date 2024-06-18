# Files are installed under $(DESTDIR)/$(PREFIX)
PREFIX ?= /usr/local
DEST := $(shell echo "$(DESTDIR)/$(PREFIX)" | sed 's:///*:/:g; s://*$$::')

GO ?= go
TAR ?= tar
ZIP ?= zip
ZSTD ?= zstd
PLANTUML ?= plantuml # may also be "java -jar plantuml.jar" if installed elsewhere

# The KCONFIG programs are only needed for re-generating the ".config" file.
# You can install the python "kconfiglib", if you don't have kconfig/kbuild.
KCONFIG_CONF ?= $(shell command -v kconfig-conf || command -v kbuild-conf || echo oldconfig)
KCONFIG_MCONF ?= $(shell command -v kconfig-mconf || command -v kbuild-mconf || echo menuconfig)

GOOS ?= $(shell $(GO) env GOOS)
ifeq ($(GOOS),windows)
bat = .bat
exe = .exe
endif

GO_BUILDTAGS ?=
ifeq ($(GOOS),darwin)
MACOS_SDK_VERSION=$(shell xcrun --show-sdk-version | cut -d . -f 1)
ifeq ($(shell test $(MACOS_SDK_VERSION) -lt 13; echo $$?),0)
# The "vz" mode needs macOS 13 SDK or later
GO_BUILDTAGS += no_vz
endif
endif

ifeq ($(GOOS),windows)
WINVER_MAJOR=$(shell powershell.exe "[System.Environment]::OSVersion.Version.Major")
ifeq ($(WINVER_MAJOR),10)
WINVER_BUILD=$(shell powershell.exe "[System.Environment]::OSVersion.Version.Build")
WINVER_BUILD_HIGH_ENOUGH=$(shell powershell.exe $(WINVER_BUILD) -ge 19041)
ifeq ($(WINVER_BUILD_HIGH_ENOUGH),False)
GO_BUILDTAGS += no_wsl
endif
endif
endif

PACKAGE := github.com/lima-vm/lima

VERSION=$(shell git describe --match 'v[0-9]*' --dirty='.m' --always --tags)
VERSION_TRIMMED := $(VERSION:v%=%)

GO_BUILD := $(GO) build -ldflags="-s -w -X $(PACKAGE)/pkg/version.Version=$(VERSION)" -tags "$(GO_BUILDTAGS)"

.NOTPARALLEL:

.PHONY: all
all: binaries manpages

.PHONY: help
help:
	@echo  '  binaries        - Build all binaries'
	@echo  '  manpages        - Build manual pages'

exe: _output/bin/limactl$(exe)

.PHONY: minimal
minimal: clean \
	_output/bin/limactl$(exe) \
	codesign \
	_output/share/lima/lima-guestagent.Linux-$(shell uname -m | sed -e s/arm64/aarch64/)
	mkdir -p _output/share/lima/templates
	cp -aL examples/default.yaml _output/share/lima/templates/

config: Kconfig
	$(KCONFIG_CONF) $<

menuconfig: Kconfig
	MENUCONFIG_STYLE=aquatic \
	$(KCONFIG_MCONF) $<

# Copy the default config, if not overridden locally
# This is done to avoid a dependency on KCONFIG tools
.config: config.mk
	cp $^ $@

-include .config

ifneq ($(CONFIG_GUESTAGENT_COMPRESS_ZSTD),y)
GO_BUILDTAGS += no_zstd
endif

HELPERS = \
	_output/bin/nerdctl.lima \
	_output/bin/apptainer.lima \
	_output/bin/docker.lima \
	_output/bin/podman.lima \
	_output/bin/kubectl.lima

ifeq ($(CONFIG_GUESTAGENT_OS_LINUX),y)
ifeq ($(CONFIG_GUESTAGENT_ARCH_X8664),y)
GUESTAGENT += \
	_output/share/lima/lima-guestagent.Linux-x86_64
endif
ifeq ($(CONFIG_GUESTAGENT_ARCH_AARCH64),y)
GUESTAGENT += \
	_output/share/lima/lima-guestagent.Linux-aarch64
endif
ifeq ($(CONFIG_GUESTAGENT_ARCH_ARMV7L),y)
GUESTAGENT += \
	_output/share/lima/lima-guestagent.Linux-armv7l
endif
ifeq ($(CONFIG_GUESTAGENT_ARCH_RISCV64),y)
GUESTAGENT += \
	_output/share/lima/lima-guestagent.Linux-riscv64
endif
endif

.PHONY: binaries
binaries: clean \
	_output/bin/lima \
	_output/bin/lima$(bat) \
	_output/bin/limactl$(exe) \
	codesign \
	$(HELPERS) \
	$(GUESTAGENT)
ifeq ($(CONFIG_GUESTAGENT_COMPRESS_ZSTD),y)
	for ga in $(GUESTAGENT); do \
	ZSTD_CLEVEL=$(CONFIG_GUESTAGENT_COMPRESS_LEVEL) $(ZSTD) --rm $$ga; done
endif
	cp -aL examples _output/share/lima/templates
ifneq ($(GOOS),windows)
	ln -sf templates _output/share/lima/examples
else
	cp -aL examples _output/share/lima/examples
endif
	mkdir -p _output/share/doc/lima
	cp -aL *.md LICENSE _output/share/doc/lima
	echo "Moved to https://github.com/lima-vm/.github/blob/main/SECURITY.md" >_output/share/doc/lima/SECURITY.md
ifneq ($(GOOS),windows)
	ln -sf ../../lima/templates _output/share/doc/lima/templates
	ln -sf templates _output/share/doc/lima/examples
else
	cp -aL examples _output/share/doc/lima/examples
	cp -aL examples _output/share/doc/lima/templates
endif
	echo $(VERSION) > _output/share/doc/lima/VERSION

.PHONY: _output/bin/lima
_output/bin/lima:
	mkdir -p _output/bin
	cp -a ./cmd/lima $@

.PHONY: _output/bin/lima.bat
_output/bin/lima.bat:
	mkdir -p _output/bin
	cp -a ./cmd/lima.bat $@

.PHONY: _output/bin/nerdctl.lima
_output/bin/nerdctl.lima:
	mkdir -p _output/bin
	cp -a ./cmd/nerdctl.lima $@

_output/bin/apptainer.lima: ./cmd/apptainer.lima
	@mkdir -p _output/bin
	cp -a $^ $@

_output/bin/docker.lima: ./cmd/docker.lima
	@mkdir -p _output/bin
	cp -a $^ $@

_output/bin/podman.lima: ./cmd/podman.lima
	@mkdir -p _output/bin
	cp -a $^ $@

_output/bin/kubectl.lima: ./cmd/kubectl.lima
	@mkdir -p _output/bin
	cp -a $^ $@

.PHONY: _output/bin/limactl$(exe)
_output/bin/limactl$(exe):
	# The hostagent must be compiled with CGO_ENABLED=1 so that net.LookupIP() in the DNS server
	# calls the native resolver library and not the simplistic version in the Go library.
	CGO_ENABLED=1 $(GO_BUILD) -o $@ ./cmd/limactl

.PHONY: _output/share/lima/lima-guestagent.Linux-x86_64
_output/share/lima/lima-guestagent.Linux-x86_64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO_BUILD) -o $@ ./cmd/lima-guestagent
	chmod 644 $@

.PHONY: _output/share/lima/lima-guestagent.Linux-aarch64
_output/share/lima/lima-guestagent.Linux-aarch64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO_BUILD) -o $@ ./cmd/lima-guestagent
	chmod 644 $@

.PHONY: _output/share/lima/lima-guestagent.Linux-armv7l
_output/share/lima/lima-guestagent.Linux-armv7l:
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 $(GO_BUILD) -o $@ ./cmd/lima-guestagent
	chmod 644 $@

.PHONY: _output/share/lima/lima-guestagent.Linux-riscv64
_output/share/lima/lima-guestagent.Linux-riscv64:
	GOOS=linux GOARCH=riscv64 CGO_ENABLED=0 $(GO_BUILD) -o $@ ./cmd/lima-guestagent
	chmod 644 $@

.PHONY: manpages
manpages: _output/bin/limactl$(exe)
	@mkdir -p _output/share/man/man1
	$< generate-doc _output/share/man/man1 \
		--output _output --prefix $(PREFIX)

.PHONY: docsy
docsy: _output/bin/limactl$(exe)
	@mkdir -p website/_output/docsy
	$< generate-doc --type docsy website/_output/docsy \
		--output _output --prefix $(PREFIX)

.PHONY: diagrams
diagrams: docs/lima-sequence-diagram.png
docs/lima-sequence-diagram.png: docs/images/lima-sequence-diagram.puml
	$(PLANTUML) ./docs/images/lima-sequence-diagram.puml

.PHONY: install
install: uninstall
	mkdir -p "$(DEST)"
	# Use tar rather than cp, for better symlink handling
	( cd _output && $(TAR) c * | $(TAR) -xv --no-same-owner -C "$(DEST)" )
	if [ "$(shell uname -s )" != "Linux" -a ! -e "$(DEST)/bin/nerdctl" ]; then ln -sf nerdctl.lima "$(DEST)/bin/nerdctl"; fi
	if [ "$(shell uname -s )" != "Linux" -a ! -e "$(DEST)/bin/apptainer" ]; then ln -sf apptainer.lima "$(DEST)/bin/apptainer"; fi

.PHONY: uninstall
uninstall:
	@test -f "$(DEST)/bin/lima" || echo "lima not found in $(DEST) prefix"
	rm -rf \
		"$(DEST)/bin/lima" \
		"$(DEST)/bin/lima$(bat)" \
		"$(DEST)/bin/limactl$(exe)" \
		"$(DEST)/bin/nerdctl.lima" \
		"$(DEST)/bin/apptainer.lima" \
		"$(DEST)/bin/docker.lima" \
		"$(DEST)/bin/podman.lima" \
		"$(DEST)/bin/kubectl.lima" \
		"$(DEST)/share/man/man1/lima.1" \
		"$(DEST)/share/man/man1/limactl"*".1" \
		"$(DEST)/share/lima" "$(DEST)/share/doc/lima"
	if [ "$$(readlink "$(DEST)/bin/nerdctl")" = "nerdctl.lima" ]; then rm "$(DEST)/bin/nerdctl"; fi
	if [ "$$(readlink "$(DEST)/bin/apptainer")" = "apptainer.lima" ]; then rm "$(DEST)/bin/apptainer"; fi

.PHONY: check-generated
check-generated:
	@test -z "$$(git status --short | grep ".pb.desc" | tee /dev/stderr)" || \
		((git diff $$(find . -name '*.pb.desc') | cat) && \
		(echo "Please run 'make generate' when making changes to proto files and check-in the generated file changes" && false))

.PHONY: lint
lint: check-generated
	golangci-lint run ./...
	yamllint .
	find . -name '*.sh' | xargs shellcheck
	find . -name '*.sh' | xargs shfmt -s -d

.PHONY: clean
clean:
	rm -rf _output vendor

.PHONY: install-tools
install-tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

.PHONY: generate
generate:
	go generate ./...

.PHONY: artifacts-darwin
artifacts-darwin:
	mkdir -p _artifacts
	GOOS=darwin GOARCH=amd64 make clean binaries
	$(TAR) -C _output/ -czvf _artifacts/lima-$(VERSION_TRIMMED)-Darwin-x86_64.tar.gz ./
	GOOS=darwin GOARCH=arm64 make clean binaries
	$(TAR) -C _output -czvf _artifacts/lima-$(VERSION_TRIMMED)-Darwin-arm64.tar.gz ./

.PHONY: artifacts-linux
artifacts-linux:
	mkdir -p _artifacts
	GOOS=linux GOARCH=amd64 CC=x86_64-linux-gnu-gcc make clean binaries
	$(TAR) -C _output/ -czvf _artifacts/lima-$(VERSION_TRIMMED)-Linux-x86_64.tar.gz ./
	GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc make clean binaries
	$(TAR) -C _output/ -czvf _artifacts/lima-$(VERSION_TRIMMED)-Linux-aarch64.tar.gz ./

.PHONY: artifacts-windows
artifacts-windows:
	mkdir -p _artifacts
	GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc make clean binaries
	$(TAR) -C _output/ -czvf _artifacts/lima-$(VERSION_TRIMMED)-Windows-x86_64.tar.gz ./
	cd _output && $(ZIP) -r ../_artifacts/lima-$(VERSION_TRIMMED)-Windows-x86_64.zip *

.PHONY: artifacts-misc
artifacts-misc:
	mkdir -p _artifacts
	go mod vendor
	$(TAR) -czf _artifacts/lima-$(VERSION_TRIMMED)-go-mod-vendor.tar.gz go.mod go.sum vendor

.PHONY: codesign
codesign:
ifeq ($(GOOS),darwin)
	codesign --entitlements vz.entitlements -s - ./_output/bin/limactl
endif
