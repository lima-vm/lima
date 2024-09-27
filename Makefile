# Files are installed under $(DESTDIR)/$(PREFIX)
PREFIX ?= /usr/local
DEST := $(shell echo "$(DESTDIR)/$(PREFIX)" | sed 's:///*:/:g; s://*$$::')

GO ?= go
TAR ?= tar
ZIP ?= zip
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

LDFLAGS := -ldflags="-s -w -X $(PACKAGE)/pkg/version.Version=$(VERSION)"
GO_BUILD := $(GO) build $(LDFLAGS) -tags "$(GO_BUILDTAGS)"

.NOTPARALLEL:

.PHONY: all
all: binaries manpages

.PHONY: help
help:
	@echo  '  binaries        - Build all binaries'
	@echo  '  manpages        - Build manual pages'
	@echo
	@echo  "  Use 'make help-targets' to see additional targets."

.PHONY: help-targets
help-targets:
	@echo  '# Targets can be categorized by their location.'
	@echo
	@echo  'Targets for files in _output/bin/:'
	@echo  '- limactl                   : Build limactl, and lima'
	@echo  '- lima                      : Copy lima, and lima.bat'
	@echo  '- helpers                   : Copy nerdctl.lima, apptainer.lima, docker.lima, podman.lima, and kubectl.lima'
	@echo
	@echo  'Targets for files in _output/share/lima/:'
	@echo  '- guestagents               : Build guestagents for archs enabled by CONFIG_GUESTAGENT_ARCHS_*'
	@echo  '- native-guestagent         : Build guestagent for native arch'
	@echo  '- additional-guestagents    : Build guestagents for archs other than native arch'
	@echo  '- <arch>-guestagent         : Build guestagent for <arch>: $(sort $(GUESTAGENT_ARCHS))'
	@echo
	@echo  'Targets for files in _output/share/lima/templates/:'
	@echo  '- templates                 : Copy templates'
	@echo  '- template_experimentals    : Copy experimental templates to experimental/'
	@echo  '- default_template          : Copy default.yaml template'
	@echo  '- create-examples-link      : Create a symlink at ../examples pointing to templates'
	@echo
	@echo  'Targets for files in _output/share/doc/lima:'
	@echo  '- documentation             : Copy documentation to _output/share/doc/lima'
	@echo  '- create-links-in-doc-dir   : Create some symlinks pointing ../../lima/templates'
	@echo
	@echo  '# e.g. to install limactl, helpers, native guestagent, and templates:'
	@echo  '#   make native install'

exe: _output/bin/limactl$(exe)

.PHONY: minimal native
minimal: clean limactl native-guestagent default_template
native: clean limactl helpers native-guestagent templates

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

.PHONY: binaries
binaries: clean \
	limactl helpers guestagents \
	templates template_experimentals create-examples-link \
	documentation create-links-in-doc-dir

# _output/bin
.PHONY: limactl lima helpers
limactl: _output/bin/limactl$(exe) codesign lima

# returns a list of files expanded from $(1) excluding directories.
glob_excluding_dir = $(shell bash -c -O extglob -O globstar -O nullglob 'for f in $(1); do test -d $$f || echo $$f; done')
FILES_IN_PKG = $(call glob_excluding_dir, ./pkg/**/!(*_test.go))

# returns a list of files which are dependencies for the command $(1).
dependencis_for_cmd = go.mod $(call glob_excluding_dir, ./cmd/$(1)/**/!(*_test.go)) $(FILES_IN_PKG)

# returns GOVERSION, CGO*, GO*, and -ldflags build variables from the output of `go version -m $(1)`.
# When CGO_* variables are not set, they are not included in the output.
# Because the CGO_* variables are not set means that those values are default values,
# it can be assumed that those values are same if the GOVERSION is same.
extract_build_vars = $(shell \
	($(GO) version -m $(1) 2>&- || echo $(1):) | \
	awk 'FNR==1{print "GOVERSION="$$2}$$2~/^(CGO|GO|-ldflags)[^=]*=[^ ]+/{sub("^.*"$$2,$$2); print $$0}' \
)

# a list of build variables that the limactl binary was built with.
limactl_build_vars = $(call extract_build_vars,_output/bin/limactl$(exe))

# a list of keys of build variables that are used in limactl_build_vars.
keys_in_limactl_build_vars = $(shell for i in $(limactl_build_vars); do echo $${i%%=*}; done)

# a list of build variables that current Go build uses.
current_go_build_vars = $(shell \
	CGO_ENABLED=1 $(GO) env $(keys_in_limactl_build_vars) | \
	awk '/ /{print "\""$$0"\""; next}{print}' | \
	for k in $(keys_in_limactl_build_vars); do read -r v && echo "$$k=$${v}"; done | \
	sed 's$$-ldflags=$$$(LDFLAGS)$$' \
)

# force build limactl if the build variables in the limactl binary is different from the current GO build variables.
ifneq ($(filter-out $(current_go_build_vars),$(limactl_build_vars)),)
.PHONY: _output/bin/limactl$(exe)
endif

# dependencies for limactl
DEPENDENCIES_FOR_LIMACTL = $(call dependencis_for_cmd,limactl)
ifeq ($(GOOS),darwin)
DEPENDENCIES_FOR_LIMACTL += vz.entitlements
endif

_output/bin/limactl$(exe): $(DEPENDENCIES_FOR_LIMACTL)
	# The hostagent must be compiled with CGO_ENABLED=1 so that net.LookupIP() in the DNS server
	# calls the native resolver library and not the simplistic version in the Go library.
	CGO_ENABLED=1 $(GO_BUILD) -o $@ ./cmd/limactl

LIMA_CMDS = lima lima$(bat)
lima: $(addprefix _output/bin/,$(LIMA_CMDS))

HELPER_CMDS = nerdctl.lima apptainer.lima docker.lima podman.lima kubectl.lima
helpers: $(addprefix _output/bin/,$(HELPER_CMDS))

_output/bin/%: ./cmd/% | _output/bin
	cp -a $< $@

MKDIR_TARGETS += _output/bin

# _output/share/lima/lima-guestagent
# How to add architecure specific guestagent:
# 1. Add the architecture to GUESTAGENT_ARCHS
# 2. Add GUESTAGENT_ARCH_ENVS_<arch> to set GOARCH and other necessary environment variables
ifeq ($(CONFIG_GUESTAGENT_OS_LINUX),y)
GUESTAGENT_ARCHS = aarch64 armv7l riscv64 x86_64
NATIVE_GUESTAGENT_ARCH = $(shell uname -m | sed -e s/arm64/aarch64/)
ADDITIONAL_GUESTAGENT_ARCHS = $(filter-out $(NATIVE_GUESTAGENT_ARCH),$(GUESTAGENT_ARCHS))

# CONFIG_GUESTAGENT_ARCH_<arch> naming convention: uppercase, remove '_'
config_guestagent_arch_name = CONFIG_GUESTAGENT_ARCH_$(shell echo $(1)|tr -d _|tr a-z A-Z)

# guestagent_path returns the path to the guestagent binary for the given architecture,
# or an empty string if the CONFIG_GUESTAGENT_ARCH_<arch> is not set.
guestagent_path = $(if $(findstring y,$($(call config_guestagent_arch_name,$(1)))),_output/share/lima/lima-guestagent.Linux-$(1))

# apply CONFIG_GUESTAGENT_ARCH_*
GUESTAGENTS = $(foreach arch,$(GUESTAGENT_ARCHS),$(call guestagent_path,$(arch)))
GUESTAGENT_ARCH_ENVS_aarch64 = GOARCH=arm64
GUESTAGENT_ARCH_ENVS_armv7l = GOARCH=arm GOARM=7
GUESTAGENT_ARCH_ENVS_riscv64 = GOARCH=riscv64
GUESTAGENT_ARCH_ENVS_x86_64 = GOARCH=amd64
NATIVE_GUESTAGENT=_output/share/lima/lima-guestagent.Linux-$(NATIVE_GUESTAGENT_ARCH)
ADDITIONAL_GUESTAGENTS=$(addprefix _output/share/lima/lima-guestagent.Linux-,$(ADDITIONAL_GUESTAGENT_ARCHS))
endif

.PHONY: guestagents native-guestagent additional-guestagents
guestagents: $(GUESTAGENTS)
native-guestagent: $(NATIVE_GUESTAGENT)
additional-guestagents: $(ADDITIONAL_GUESTAGENTS)
%-guestagent:
	@[ "$(findstring $(*),$(GUESTAGENT_ARCHS))" == "$(*)" ] && make _output/share/lima/lima-guestagent.Linux-$*
_output/share/lima/lima-guestagent.Linux-%: $(call dependencis_for_cmd,lima-guestagent) | _output/share/lima
	GOOS=linux $(GUESTAGENT_ARCH_ENVS_$*) CGO_ENABLED=0 $(GO_BUILD) -o $@ ./cmd/lima-guestagent
	chmod 644 $@
ifeq ($(CONFIG_GUESTAGENT_COMPRESS),y)
	gzip $@
endif

MKDIR_TARGETS += _output/share/lima

# _output/share/lima/templates
TEMPLATES=$(addprefix _output/share/lima/templates/,$(filter-out experimental,$(notdir $(wildcard examples/*))))
TEMPLATE_EXPERIMENTALS=$(addprefix _output/share/lima/templates/experimental/,$(notdir $(wildcard examples/experimental/*)))

.PHONY: default_template templates template_experimentals
default_template: _output/share/lima/templates/default.yaml
templates: $(TEMPLATES)
template_experimentals: $(TEMPLATE_EXPERIMENTALS)

$(TEMPLATES): | _output/share/lima/templates
$(TEMPLATE_EXPERIMENTALS): | _output/share/lima/templates/experimental
MKDIR_TARGETS += _output/share/lima/templates _output/share/lima/templates/experimental

_output/share/lima/templates/%: examples/%
	cp -aL $< $@


# _output/share/lima/examples
.PHONY: create-examples-link
create-examples-link: _output/share/lima/examples
_output/share/lima/examples: default_template # depends on minimal template target
ifneq ($(GOOS),windows)
	ln -sf templates $@
else
# copy from templates builded in build process
	cp -aL _output/share/lima/templates $@
endif

# _output/share/doc/lima
DOCUMENTATION=$(addprefix _output/share/doc/lima/,$(wildcard *.md) LICENSE SECURITY.md VERSION)

.PHONY: documentation
documentation: $(DOCUMENTATION)

_output/share/doc/lima/SECURITY.md: | _output/share/doc/lima
	echo "Moved to https://github.com/lima-vm/.github/blob/main/SECURITY.md" > $@

_output/share/doc/lima/VERSION: | _output/share/doc/lima
	echo $(VERSION) > $@

_output/share/doc/lima/%: % | _output/share/doc/lima
	cp -aL $< $@

MKDIR_TARGETS += _output/share/doc/lima


.PHONY: create-links-in-doc-dir
create-links-in-doc-dir: _output/share/doc/lima/templates _output/share/doc/lima/examples
_output/share/doc/lima/templates: default_template # depends on minimal template target
ifneq ($(GOOS),windows)
	ln -sf ../../lima/templates $@
else
# copy from templates builded in build process
	cp -aL _output/share/lima/templates $@
endif
_output/share/doc/lima/examples: default_template # depends on minimal template target
ifneq ($(GOOS),windows)
	ln -sf templates $@
else
# copy from templates builded in build process
	cp -aL _output/share/lima/templates $@
endif

# _output/share/man/man1
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
	ls-lint
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

.PHONY: artifacts-darwin artifact-darwin-arm64 artifact-darwin-x86_64
artifacts-darwin: artifact-darwin-x86_64 artifact-darwin-arm64
artifact-darwin-arm64: ENVS=GOOS=darwin GOARCH=arm64
artifact-darwin-arm64: _artifacts/lima-$(VERSION_TRIMMED)-Darwin-arm64.tar.gz
artifact-darwin-x86_64: ENVS=GOOS=darwin GOARCH=amd64
artifact-darwin-x86_64: _artifacts/lima-$(VERSION_TRIMMED)-Darwin-x86_64.tar.gz

.PHONY: artifacts-linux artifact-linux-aarch64 artifact-linux-x86_64
artifacts-linux: artifact-linux-x86_64 artifact-linux-aarch64
artifact-linux-aarch64: ENVS=GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc
artifact-linux-aarch64: _artifacts/lima-$(VERSION_TRIMMED)-Linux-aarch64.tar.gz
artifact-linux-x86_64: ENVS=GOOS=linux GOARCH=amd64 CC=x86_64-linux-gnu-gcc
artifact-linux-x86_64: _artifacts/lima-$(VERSION_TRIMMED)-Linux-x86_64.tar.gz

.PHONY: artifacts-windows artifact-windows-x86_64
artifacts-windows: artifact-windows-x86_64
artifact-windows-x86_64: ENVS=GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc
artifact-windows-x86_64: _artifacts/lima-$(VERSION_TRIMMED)-Windows-x86_64.tar.gz
	cd _output && $(ZIP) -r ../_artifacts/lima-$(VERSION_TRIMMED)-Windows-x86_64.zip *

_artifacts/lima-%.tar.gz: | _artifacts
	$(ENVS) make clean binaries
	$(TAR) -C _output/ -czvf $@ ./

MKDIR_TARGETS += _artifacts

.PHONY: artifacts-misc
artifacts-misc: | _artifacts
	go mod vendor
	$(TAR) -czf _artifacts/lima-$(VERSION_TRIMMED)-go-mod-vendor.tar.gz go.mod go.sum vendor

.PHONY: codesign
codesign:
ifeq ($(GOOS),darwin)
	codesign --entitlements vz.entitlements -s - ./_output/bin/limactl
endif

# This target must be placed after any changes to the `MKDIR_TARGETS` variable.
# It seems that variable expansion in Makefile targets is not done recursively.
$(MKDIR_TARGETS):
	mkdir -p $@
