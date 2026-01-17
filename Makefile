# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0
# Files are installed under $(DESTDIR)/$(PREFIX)
PREFIX ?= /usr/local
DEST := $(shell echo "$(DESTDIR)/$(PREFIX)" | sed 's:///*:/:g; s://*$$::')

GO ?= go
TAR ?= tar
ZIP ?= zip
PLANTUML ?= plantuml # may also be "java -jar plantuml.jar" if installed elsewhere

GOARCH ?= $(shell $(GO) env GOARCH)
GOHOSTARCH := $(shell $(GO) env GOHOSTARCH)
GOHOSTOS := $(shell $(GO) env GOHOSTOS)
GOOS ?= $(shell $(GO) env GOOS)
ifeq ($(GOOS),windows)
bat = .bat
exe = .exe
endif

ifeq ($(GOOS),darwin)
MACOS_SDK_VERSION = $(shell xcrun --show-sdk-version | cut -d . -f 1)
# xcrun command seems to fail even when the SDK is available:
# > xcrun: error: unable to lookup item 'SDKVersion' in SDK '/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk'
ifeq ($(MACOS_SDK_VERSION),)
MACOS_SDK_VERSION = $(shell readlink /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk | sed -E -e "s/^MacOSX//" -e "s/\.[0-9]+\.sdk//")
endif
endif

DEFAULT_ADDITIONAL_DRIVERS :=
ifeq ($(GOOS),darwin)
ifeq ($(GOARCH),arm64)
ifeq ($(shell test $(MACOS_SDK_VERSION) -ge 14; echo $$?),0)
# krunkit needs macOS 14 or later: https://github.com/containers/libkrun/blob/main/README.md#macos-efi-variant
DEFAULT_ADDITIONAL_DRIVERS += krunkit
endif
endif
endif
ADDITIONAL_DRIVERS ?= $(DEFAULT_ADDITIONAL_DRIVERS)

GO_BUILDTAGS ?=
ifeq ($(GOOS),darwin)
ifeq ($(shell test $(MACOS_SDK_VERSION) -lt 13; echo $$?),0)
# The "vz" mode needs macOS 13 SDK or later
GO_BUILDTAGS += no_vz
endif
endif

ifeq ($(GOHOSTOS),windows)
WINVER_MAJOR=$(shell powershell.exe "[System.Environment]::OSVersion.Version.Major")
ifeq ($(WINVER_MAJOR),10)
WINVER_BUILD=$(shell powershell.exe "[System.Environment]::OSVersion.Version.Build")
WINVER_BUILD_HIGH_ENOUGH=$(shell powershell.exe $(WINVER_BUILD) -ge 19041)
ifeq ($(WINVER_BUILD_HIGH_ENOUGH),False)
GO_BUILDTAGS += no_wsl
endif
endif
endif

PACKAGE := github.com/lima-vm/lima/v2

VERSION := $(shell git describe --match 'v[0-9]*' --dirty='.m' --always --tags)
VERSION_TRIMMED := $(VERSION:v%=%)

# `DEBUG` flag to build binaries with debug information for use by `dlv exec`.
# This implies KEEP_DWARF=1 and KEEP_SYMBOLS=1.
DEBUG ?=
GO_BUILD_GCFLAGS ?=
KEEP_DWARF ?=
KEEP_SYMBOLS ?=
ifeq ($(DEBUG),1)
	# Disable optimizations and inlining to make debugging easier.
	GO_BUILD_GCFLAGS = -gcflags="all=-N -l"
	# Keep the symbol table
	KEEP_DWARF = 1
	# Enable DWARF generation
	KEEP_SYMBOLS = 1
endif

GO_BUILD_LDFLAGS_W := true
ifeq ($(KEEP_DWARF),1)
	GO_BUILD_LDFLAGS_W = false
endif

GO_BUILD_LDFLAGS_S := true
ifeq ($(KEEP_SYMBOLS),1)
	GO_BUILD_LDFLAGS_S = false
endif
# `-s`: Strip the symbol table according to the KEEP_SYMBOLS config
# `-w`: Disable DWARF generation according to the KEEP_DWARF config
# `-X`: Embed version information.
GO_BUILD_LDFLAGS := -ldflags="-s=$(GO_BUILD_LDFLAGS_S) -w=$(GO_BUILD_LDFLAGS_W) -X $(PACKAGE)/pkg/version.Version=$(VERSION)"
# `go -version -m` returns -tags with comma-separated list, because space-separated list is deprecated in go1.13.
# converting to comma-separated list is useful for comparing with the output of `go version -m`.
GO_BUILD_FLAG_TAGS := $(addprefix -tags=,$(shell echo "$(GO_BUILDTAGS)"|tr " " "\n"|paste -sd "," -))
GO_BUILD := $(strip $(GO) build $(GO_BUILD_GCFLAGS) $(GO_BUILD_LDFLAGS) $(GO_BUILD_FLAG_TAGS))

################################################################################
# Features
.NOTPARALLEL:
.SECONDEXPANSION:

################################################################################
# Build binaries and manpages
.PHONY: all
all: binaries manpages

################################################################################
##@ Core
# Show this help message
.PHONY: help
help:
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Variables (can be overridden):'
	@echo '  PREFIX        Installation prefix (default: /usr/local)'
	@echo '  KEEP_DWARF    Keep DWARF information (1 or 0, default: 0)'
	@echo '  KEEP_SYMBOLS  Keep symbols (1 or 0, default: 0)'
	@echo '  DEBUG         Build with debug information (1 or 0, default: 0)'
	@echo ''
	@awk '\
	BEGIN { desc="" } \
	/^##@/ { \
		printf "\n\033[1m%s\033[0m\n", substr($$0, 5); \
		next \
	} \
	/^# *@nohelp/ { \
		skip=1; \
		next \
	} \
	/^# / { \
		desc = substr($$0, 3); \
		next \
	} \
	/^[a-zA-Z_0-9\/-]+:/ { \
		target = $$1; \
		sub(/:.*/, "", target); \
		if (!skip && !seen[target]++) { \
			printf "  \033[36m%-40s\033[0m %s\n", target, (desc ? desc : "(no description)"); \
		} \
		desc = ""; skip=0; \
	} \
	' $(MAKEFILE_LIST)
################################################################################
# convenience targets
exe: _output/bin/limactl$(exe)

##@ Build Presets
.PHONY: minimal native

# Predefined minimal build
minimal: clean limactl native-guestagent default_template

# Build using native configuration
native: clean limactl limactl-plugins helpers native-guestagent templates template_experimentals additional-drivers

################################################################################
# These configs were once customizable but should no longer be changed.
CONFIG_GUESTAGENT_OS_LINUX=y
CONFIG_GUESTAGENT_ARCH_X8664=y
CONFIG_GUESTAGENT_ARCH_AARCH64=y
CONFIG_GUESTAGENT_ARCH_ARMV7L=y
CONFIG_GUESTAGENT_ARCH_PPC64LE=y
CONFIG_GUESTAGENT_ARCH_RISCV64=y
CONFIG_GUESTAGENT_ARCH_S390X=y
CONFIG_GUESTAGENT_COMPRESS=y

################################################################################
# Legacy binary build configuration (do not modify)
.PHONY: binaries
binaries: limactl helpers limactl-plugins guestagents \
	templates template_experimentals \
	documentation create-links-in-doc-dir

################################################################################
##@ Binaries
# Build limactl binary (_output/bin)
.PHONY: limactl lima helpers
limactl: _output/bin/limactl$(exe) lima

### Listing Dependencies

# returns a list of files expanded from $(1) excluding directories and files ending with '_test.go'.
find_files_excluding_dir_and_test = $(shell find $(1) ! -type d ! -name '*_test.go')
FILES_IN_PKG = $(call find_files_excluding_dir_and_test, ./pkg)

# returns a list of files which are dependencies for the command $(1).
dependencies_for_cmd = go.mod $(call find_files_excluding_dir_and_test, ./cmd/$(1)) $(FILES_IN_PKG)

### Force Building Targets

# returns GOVERSION, CGO*, GO*, -ldflags, and -tags build variables from the output of `go version -m $(1)`.
# When CGO_* variables are not set, they are not included in the output.
# Because the CGO_* variables are not set means that those values are default values,
# it can be assumed that those values are same if the GOVERSION is same.
# $(1): target binary
extract_build_vars = $(shell \
	($(GO) version -m $(1) 2>&- || echo $(1):) | \
	awk 'FNR==1{print "GOVERSION="$$2}$$2~/^(CGO|GO|-gcflags|-ldflags|-tags).*=.+$$/{sub("^.*"$$2,$$2); print $$0}' \
)

# a list of keys from the GO build variables to be used for calling `go env`.
# keys starting with '-' are excluded because `go env` does not support those keys.
# $(1): extracted build variables from the binary
keys_in_build_vars = $(filter-out -%,$(shell for i in $(1); do echo $${i%%=*}; done))

# a list of GO build variables to build the target binary.
# $(1): target binary. expecting ENVS_$(2) is set to use the environment variables for the target binary.
# $(2): key of the GO build variable to be used for calling `go env`.
go_build_vars = $(shell \
	$(ENVS_$(1)) $(GO) env $(2) | \
	awk '/ /{print "\""$$0"\""; next}{print}' | \
	for k in $(2); do read -r v && echo "$$k=$${v}"; done \
) $(GO_BUILD_GCFLAGS) $(GO_BUILD_LDFLAGS) $(GO_BUILD_FLAG_TAGS)

# returns the difference between $(1) and $(2).
diff = $(filter-out $(2),$(1))$(filter-out $(1),$(2))

# returns diff between the GO build variables in the binary $(1) and the building variables.
# $(1): target binary
# $(2): extracted GO build variables from the binary
compare_build_vars = $(call diff,$(call go_build_vars,$(1),$(call keys_in_build_vars,$(2))),$(2))

# returns "force" if the GO build variables in the binary $(1) is different from the building variables.
# $(1): target binary. expecting ENVS_$(1) is set to use the environment variables for the target binary.
force_build = $(if $(call compare_build_vars,$(1),$(call extract_build_vars,$(1))),force,)

# returns the file name without .gz extension. It also gunzips the file with .gz extension if exists.
# $(1): target file
gunzip_if_exists = $(shell f=$(1); f=$${f%.gz}; test -f "$${f}.gz" && (set -x; gunzip -f "$${f}.gz") ; echo "$${f}")

# call force_build with passing output of gunzip_if_exists as an argument.
# $(1): target file
force_build_with_gunzip = $(call force_build,$(call gunzip_if_exists,$(1)))

#$(1): target file
.PHONY: force
force: # placeholder for force build

################################################################################
# _output/bin/limactl$(exe)

# dependencies for limactl
LIMACTL_DEPS = $(call dependencies_for_cmd,limactl)
ifeq ($(GOOS),darwin)
LIMACTL_DEPS += vz.entitlements
endif

# environment variables for limactl. this variable is used for checking force build.
#
# The hostagent must be compiled with CGO_ENABLED=1 so that net.LookupIP() in the DNS server
# calls the native resolver library and not the simplistic version in the Go library.
ENVS__output/bin/limactl$(exe) = CGO_ENABLED=1 GOOS="$(GOOS)" GOARCH="$(GOARCH)" CC="$(CC)"

LIMACTL_DRIVER_TAGS :=
ifneq (,$(findstring vz,$(ADDITIONAL_DRIVERS)))
LIMACTL_DRIVER_TAGS += external_vz
endif
ifneq (,$(findstring qemu,$(ADDITIONAL_DRIVERS)))
LIMACTL_DRIVER_TAGS += external_qemu
endif
ifneq (,$(findstring wsl2,$(ADDITIONAL_DRIVERS)))
LIMACTL_DRIVER_TAGS += external_wsl2
endif

GO_BUILDTAGS ?=
GO_BUILDTAGS_LIMACTL := $(strip $(GO_BUILDTAGS) $(LIMACTL_DRIVER_TAGS))

_output/bin/limactl$(exe): $(LIMACTL_DEPS) $$(call force_build,$$@)
ifneq ($(GOOS),windows) #
	@rm -rf _output/bin/limactl.exe
else
	@rm -rf _output/bin/limactl
endif
	$(ENVS_$@) $(GO_BUILD) -tags '$(GO_BUILDTAGS_LIMACTL)' -o $@ ./cmd/limactl
ifeq ($(GOOS),darwin)
	codesign -f -v --entitlements vz.entitlements -s - $@
endif

LIBEXEC_LIMA := _output/libexec/lima

# Build limactl plugins
limactl-plugins: $(LIBEXEC_LIMA)/limactl-mcp$(exe)

$(LIBEXEC_LIMA)/limactl-mcp$(exe): $(call dependencies_for_cmd,limactl-mcp) $$(call force_build,$$@)
	@mkdir -p $(LIBEXEC_LIMA)
	$(ENVS_$@) $(GO_BUILD) -o $@ ./cmd/limactl-mcp

##@ Drivers
# Build additional drivers
.PHONY: additional-drivers
additional-drivers:
	@mkdir -p $(LIBEXEC_LIMA)
	@for drv in $(ADDITIONAL_DRIVERS); do \
		echo "Building $$drv as external"; \
		if [ "$(GOOS)" = "windows" ]; then \
			$(GO_BUILD) -o $(LIBEXEC_LIMA)/lima-driver-$$drv.exe ./cmd/lima-driver-$$drv; \
		else \
			$(GO_BUILD) -o $(LIBEXEC_LIMA)/lima-driver-$$drv ./cmd/lima-driver-$$drv; \
			fi; \
		if [ "$$drv" = "vz" ] && [ "$(GOOS)" = "darwin" ]; then \
			codesign -f -v --entitlements vz.entitlements -s - $(LIBEXEC_LIMA)/lima-driver-vz; \
		fi; \
	done

LIMA_CMDS = $(sort lima lima$(bat)) # $(sort ...) deduplicates the list
LIMA_DEPS = $(addprefix _output/bin/,$(LIMA_CMDS))

##@ Lima
# Build core lima components
lima: $(LIMA_DEPS)

HELPER_CMDS = nerdctl.lima apptainer.lima docker.lima podman.lima kubectl.lima
HELPERS_DEPS = $(addprefix _output/bin/,$(HELPER_CMDS))

# Build helper utilities
helpers: $(HELPERS_DEPS)

_output/bin/%: ./cmd/% | _output/bin
	cp -a $< $@

MKDIR_TARGETS += _output/bin

################################################################################
# _output/share/lima/lima-guestagent
LINUX_GUESTAGENT_PATH_COMMON = _output/share/lima/lima-guestagent.Linux-

# How to add architecture specific guestagent:
# 1. Add the architecture to GUESTAGENT_ARCHS
# 2. Add ENVS_$(*_GUESTAGENT_PATH_COMMON)<arch> to set GOOS, GOARCH, and other necessary environment variables
LINUX_GUESTAGENT_ARCHS = aarch64 armv7l ppc64le riscv64 s390x x86_64

ifeq ($(CONFIG_GUESTAGENT_OS_LINUX),y)
ALL_GUESTAGENTS_NOT_COMPRESSED += $(addprefix $(LINUX_GUESTAGENT_PATH_COMMON),$(LINUX_GUESTAGENT_ARCHS))
endif
ifeq ($(CONFIG_GUESTAGENT_COMPRESS),y)
$(info Guestagents are unzipped each time to check the build configuration; they may be zipped afterward.)
gz=.gz
endif

ALL_GUESTAGENTS = $(addsuffix $(gz),$(ALL_GUESTAGENTS_NOT_COMPRESSED))

# guestagent path for the given platform. it may has .gz extension if CONFIG_GUESTAGENT_COMPRESS is enabled.
# $(1): operating system (os)
# $(2): list of architectures
guestagent_path = $(foreach arch,$(2),$($(1)_GUESTAGENT_PATH_COMMON)$(arch)$(gz))

ifeq ($(CONFIG_GUESTAGENT_OS_LINUX),y)
NATIVE_GUESTAGENT_ARCH = $(shell echo $(GOARCH) | sed -e s/arm64/aarch64/ -e s/arm/armv7l/ -e s/amd64/x86_64/)
NATIVE_GUESTAGENT = $(call guestagent_path,LINUX,$(NATIVE_GUESTAGENT_ARCH))
ADDITIONAL_GUESTAGENT_ARCHS = $(filter-out $(NATIVE_GUESTAGENT_ARCH),$(LINUX_GUESTAGENT_ARCHS))
ADDITIONAL_GUESTAGENTS = $(call guestagent_path,LINUX,$(ADDITIONAL_GUESTAGENT_ARCHS))
endif

# config_guestagent_arch returns expanded value of CONFIG_GUESTAGENT_ARCH_<arch>
# $(1): architecture
# CONFIG_GUESTAGENT_ARCH_<arch> naming convention: uppercase, remove '_'
config_guestagent_arch = $(filter y,$(CONFIG_GUESTAGENT_ARCH_$(shell echo $(1)|tr -d _|tr a-z A-Z)))

# guestagent_path_enabled_by_config returns the path to the guestagent binary for the given architecture,
# or an empty string if the CONFIG_GUESTAGENT_ARCH_<arch> is not set.
guestagent_path_enabled_by_config = $(if $(call config_guestagent_arch,$(2)),$(call guestagent_path,$(1),$(2)))

ifeq ($(CONFIG_GUESTAGENT_OS_LINUX),y)
# apply CONFIG_GUESTAGENT_ARCH_*
GUESTAGENTS += $(foreach arch,$(LINUX_GUESTAGENT_ARCHS),$(call guestagent_path_enabled_by_config,LINUX,$(arch)))
endif

##@ Guest Agents
# Build guest agents (apply CONFIG_GUESTAGENT_ARCH_*)
.PHONY: guestagents native-guestagent additional-guestagents
guestagents: $(GUESTAGENTS)
# Build native guest agent
native-guestagent: $(NATIVE_GUESTAGENT)
# Build additional guest agents
additional-guestagents: $(ADDITIONAL_GUESTAGENTS)
%-guestagent:
	@[ "$(findstring $(*),$(LINUX_GUESTAGENT_ARCHS))" == "$(*)" ] && make $(call guestagent_path,LINUX,$*)

# environment variables for linux-guestagent. these variable are used for checking force build.
ENVS_$(LINUX_GUESTAGENT_PATH_COMMON)aarch64 = CGO_ENABLED=0 GOOS=linux GOARCH=arm64
ENVS_$(LINUX_GUESTAGENT_PATH_COMMON)armv7l = CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7
ENVS_$(LINUX_GUESTAGENT_PATH_COMMON)ppc64le = CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le
ENVS_$(LINUX_GUESTAGENT_PATH_COMMON)riscv64 = CGO_ENABLED=0 GOOS=linux GOARCH=riscv64
ENVS_$(LINUX_GUESTAGENT_PATH_COMMON)s390x = CGO_ENABLED=0 GOOS=linux GOARCH=s390x
ENVS_$(LINUX_GUESTAGENT_PATH_COMMON)x86_64 = CGO_ENABLED=0 GOOS=linux GOARCH=amd64
$(ALL_GUESTAGENTS_NOT_COMPRESSED): $(call dependencies_for_cmd,lima-guestagent) $$(call force_build_with_gunzip,$$@) | _output/share/lima
	$(ENVS_$@) $(GO_BUILD) -o $@ ./cmd/lima-guestagent
	chmod 644 $@
$(LINUX_GUESTAGENT_PATH_COMMON)%.gz: $(LINUX_GUESTAGENT_PATH_COMMON)% $$(call force_build_with_gunzip,$$@)
	@set -x; gzip $<

MKDIR_TARGETS += _output/share/lima

################################################################################
# _output/share/lima/templates
TEMPLATES = $(addprefix _output/share/lima/templates/,$(filter-out experimental,$(notdir $(wildcard templates/*))))
TEMPLATE_DEFAULTS = ${addprefix _output/share/lima/templates/_default/,$(notdir $(wildcard templates/_default/*))}
TEMPLATE_IMAGES = $(addprefix _output/share/lima/templates/_images/,$(notdir $(wildcard templates/_images/*)))
TEMPLATE_EXPERIMENTALS = $(addprefix _output/share/lima/templates/experimental/,$(notdir $(wildcard templates/experimental/*)))

##@ Templates
.PHONY: default_template templates template_experimentals
# Install default templates
default_template: _output/share/lima/templates/default.yaml

# Install all templates
templates: $(TEMPLATES) $(TEMPLATE_DEFAULTS) $(TEMPLATE_IMAGES)

# Install experimental templates
template_experimentals: $(TEMPLATE_EXPERIMENTALS)

$(TEMPLATES): | _output/share/lima/templates
$(TEMPLATE_DEFAULTS): | _output/share/lima/templates/_default
$(TEMPLATE_IMAGES): | _output/share/lima/templates/_images
$(TEMPLATE_EXPERIMENTALS): | _output/share/lima/templates/experimental
MKDIR_TARGETS += _output/share/lima/templates _output/share/lima/templates/_default _output/share/lima/templates/_images _output/share/lima/templates/experimental

_output/share/lima/templates/%: templates/%
	cp -aL $< $@

# returns "force" if GOOS==windows, or GOOS!=windows and the file $(1) is not a symlink.
# $(1): target file
# On Windows, always copy to ensure the target has the same file as the source.
force_link = $(if $(filter windows,$(GOOS)),force,$(shell test ! -L $(1) && echo force))

################################################################################
# templates/_images

# fedora-N.yaml should not be updated to refer to Fedora N+1 images
TEMPLATES_TO_BE_UPDATED = $(filter-out $(wildcard templates/_images/fedora*.yaml),$(wildcard templates/_images/*.yaml))

# Update template images (fedora-N.yaml guarded)
.PHONY: update-templates
update-templates: $(TEMPLATES_TO_BE_UPDATED)
	./hack/update-template.sh $^

################################################################################
# _output/share/doc/lima
DOCUMENTATION = $(addprefix _output/share/doc/lima/,$(wildcard *.md) LICENSE SECURITY.md VERSION)

##@ Documentation
# Generate and install documentation files under _output/share/doc/lima
.PHONY: documentation
documentation: $(DOCUMENTATION)

# Internal: SECURITY.md placeholder redirect
_output/share/doc/lima/SECURITY.md: | _output/share/doc/lima
	echo "Moved to https://github.com/lima-vm/.github/blob/main/SECURITY.md" > $@

# Internal: write version file for documentation
# @nohelp
_output/share/doc/lima/VERSION: | _output/share/doc/lima
	echo $(VERSION) > $@

# Internal: copy documentation assets into output directory
# @nohelp
_output/share/doc/lima/%: % | _output/share/doc/lima
	cp -aL $< $@

MKDIR_TARGETS += _output/share/doc/lima

# Create symlinks (or copies on Windows) for templates inside documentation directory
.PHONY: create-links-in-doc-dir
create-links-in-doc-dir: _output/share/doc/lima/templates
# @nohelp
_output/share/doc/lima/templates: _output/share/lima/templates $$(call force_link,$$@)
# remove the existing directory or symlink
	rm -rf $@
ifneq ($(GOOS),windows)
	ln -sf ../../lima/templates $@
else
# copy from templates built in build process
	cp -aL $< $@
endif

################################################################################
# returns difference between GOOS GOARCH and GOHOSTOS GOHOSTARCH.
cross_compiling = $(call diff,$(GOOS) $(GOARCH),$(GOHOSTOS) $(GOHOSTARCH))
# returns true if cross_compiling is empty.
native_compiling = $(if $(cross_compiling),,true)

################################################################################
# Generate manpages using limactl (native binary required)
.PHONY: manpages
manpages: _output/share/man/man1/limactl.1
_output/share/man/man1/limactl.1: _output/bin/limactl$(exe)
	@mkdir -p _output/share/man/man1
ifeq ($(native_compiling),true)
# The manpages are generated by limactl, so the limactl binary must be native.
	$< generate-doc _output/share/man/man1 \
		--output _output --prefix $(PREFIX)
endif

################################################################################
# Generate Docsy documentation pages using limactl
.PHONY: docsy
docsy: website/_output/docsy/limactl.md website/_output/docsy-mcp/mcp.md
website/_output/docsy/limactl.md: _output/bin/limactl$(exe)
	@mkdir -p website/_output/docsy
ifeq ($(native_compiling),true)
# The docs are generated by limactl, so the limactl binary must be native.
	$< generate-doc --type docsy website/_output/docsy \
		--output _output --prefix $(PREFIX)
endif
website/_output/docsy-mcp/mcp.md: _output/libexec/lima/limactl-mcp$(exe)
	@mkdir -p website/_output/docsy-mcp
ifeq ($(native_compiling),true)
	$< generate-doc website/_output/docsy-mcp
endif

################################################################################
# Internal: generate embedded default template
default-template.yaml: _output/bin/limactl$(exe)
ifeq ($(native_compiling),true)
	$< tmpl copy --embed-all templates/default.yaml $@
endif

# Internal: generate JSON schema for lima YAML templates
schema-limayaml.json: _output/bin/limactl$(exe) templates/default.yaml default-template.yaml
ifeq ($(native_compiling),true)
	# validate both the original template (with the "base" etc), and the embedded template
	$< generate-jsonschema --schemafile $@ templates/default.yaml default-template.yaml
endif

.PHONY: check-jsonschema
check-jsonschema: schema-limayaml.json templates/default.yaml default-template.yaml
	check-jsonschema --schemafile schema-limayaml.json templates/default.yaml default-template.yaml

################################################################################
# Generate project diagrams (PlantUML)
.PHONY: diagrams
diagrams: docs/lima-sequence-diagram.png
docs/lima-sequence-diagram.png: docs/images/lima-sequence-diagram.puml
	$(PLANTUML) ./docs/images/lima-sequence-diagram.puml

################################################################################
##@ Install
# Install lima binaries, documentation, and supporting files
.PHONY: install
install: uninstall
	mkdir -p "$(DEST)"
	# Use tar rather than cp, for better symlink handling
	( cd _output && $(TAR) c * | $(TAR) -xv --no-same-owner -C "$(DEST)" )
	if [ "$(shell uname -s )" != "Linux" -a ! -e "$(DEST)/bin/nerdctl" ]; then ln -sf nerdctl.lima "$(DEST)/bin/nerdctl"; fi
	if [ "$(shell uname -s )" != "Linux" -a ! -e "$(DEST)/bin/apptainer" ]; then ln -sf apptainer.lima "$(DEST)/bin/apptainer"; fi

# Uninstall lima binaries, documentation, and supporting files
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
		"$(DEST)/share/lima" \
		"$(DEST)/share/doc/lima" \
		"$(DEST)/libexec/lima/limactl-mcp$(exe)" \
		"$(DEST)/libexec/lima/lima-driver-qemu$(exe)" \
		"$(DEST)/libexec/lima/lima-driver-vz$(exe)" \
		"$(DEST)/libexec/lima/lima-driver-wsl2$(exe)" \
		"$(DEST)/libexec/lima/lima-driver-krunkit$(exe)"
	if [ "$$(readlink "$(DEST)/bin/nerdctl")" = "nerdctl.lima" ]; then rm "$(DEST)/bin/nerdctl"; fi
	if [ "$$(readlink "$(DEST)/bin/apptainer")" = "apptainer.lima" ]; then rm "$(DEST)/bin/apptainer"; fi

##@ Validation & CI
# Verify generated files are committed and up to date
.PHONY: check-generated
check-generated:
	git diff --exit-code || \
		(echo "Please run 'make generate' when making changes to proto files and check-in the generated file changes" && false)

# Run BATS test suite
.PHONY: bats
bats: native limactl-plugins
	PATH=$$PWD/_output/bin:$$PATH ./hack/bats/lib/bats-core/bin/bats --timing ./hack/bats/tests

# Run linters and license checks
.PHONY: lint
lint: check-generated
	golangci-lint run ./...
	yamllint .
	ls-lint
	find . -name '*.sh' ! -path "./.git/*" | xargs shellcheck
	find . -name '*.sh' ! -path "./.git/*" | xargs shfmt -s -d
	go-licenses check --include_tests ./... --allowed_licenses=$$(cat ./hack/allowed-licenses.txt)
	ltag -t ./hack/ltag --check -v
	protolint .

##@ Maintenance
# Remove build artifacts and vendor directory
.PHONY: clean
clean:
	rm -rf _output vendor

# Install protoc and gRPC code generation tools
.PHONY: install-protoc-tools
install-protoc-tools:
	go install -modfile=./hack/tools/go.mod google.golang.org/protobuf/cmd/protoc-gen-go
	go install -modfile=./hack/tools/go.mod google.golang.org/grpc/cmd/protoc-gen-go-grpc

# Run go generate across the repository
.PHONY: generate
generate:
	go generate ./...

################################################################################
##@ Artifacts
# Build release artifact for current GOOS and GOARCH
.PHONY: artifact

# returns the capitalized string of $(1).
capitalize = $(shell echo "$(1)"|awk '{print toupper(substr($$0,1,1)) tolower(substr($$0,2))}')

# returns the architecture name converted from GOARCH to GNU coreutils uname -m.
to_uname_m = $(foreach arch,$(1),$(shell echo $(arch) | sed 's/amd64/x86_64/' | sed 's/arm64/aarch64/'))

ARTIFACT_FILE_EXTENSIONS := .tar.gz

ifeq ($(GOOS),darwin)
# returns the architecture name converted from GOARCH to macOS's uname -m.
to_uname_m = $(foreach arch,$(1),$(shell echo $(arch) | sed 's/amd64/x86_64/'))
else ifeq ($(GOOS),linux)
# CC is required for cross-compiling on Linux.
# On Debian, Ubuntu, and related distributions, compilers are named like x86_64-linux-gnu-gcc
# On Fedora, RHEL, and related distributions, the equivalent is x86_64-redhat-linux-gcc
# On openSUSE and as a generic fallback, gcc is used
CC := $(shell \
	if command -v $(call to_uname_m,$(GOARCH))-redhat-linux-gcc >/dev/null 2>&1; then \
		echo $(call to_uname_m,$(GOARCH))-redhat-linux-gcc; \
	elif command -v $(call to_uname_m,$(GOARCH))-linux-gnu-gcc >/dev/null 2>&1; then \
		echo $(call to_uname_m,$(GOARCH))-linux-gnu-gcc; \
	else \
		echo gcc; \
	fi)
else ifeq ($(GOOS),windows)
# Artifact in zip format is also provided for Windows
ARTIFACT_FILE_EXTENSIONS += .zip
endif

# artifacts: artifacts-$(GOOS)
ARTIFACT_OS = $(call capitalize,$(GOOS))
ARTIFACT_UNAME_M = $(call to_uname_m,$(GOARCH))
ARTIFACT_PATH_COMMON = _artifacts/lima-$(VERSION_TRIMMED)-$(ARTIFACT_OS)-$(ARTIFACT_UNAME_M)
ARTIFACT_ADDITIONAL_GUESTAGENTS_PATH_COMMON = _artifacts/lima-additional-guestagents-$(VERSION_TRIMMED)-$(ARTIFACT_OS)-$(ARTIFACT_UNAME_M)

# Build release artifacts (alias: artifacts-$(GOOS))
artifact: $(addprefix $(ARTIFACT_PATH_COMMON),$(ARTIFACT_FILE_EXTENSIONS)) \
	$(addprefix $(ARTIFACT_ADDITIONAL_GUESTAGENTS_PATH_COMMON),$(ARTIFACT_FILE_EXTENSIONS))

# Files included in release artifacts
ARTIFACT_DES =  _output/bin/limactl$(exe) limactl-plugins $(LIMA_DEPS) $(HELPERS_DEPS) \
	$(NATIVE_GUESTAGENT) \
	$(TEMPLATES) $(TEMPLATE_IMAGES) $(TEMPLATE_DEFAULTS) $(TEMPLATE_EXPERIMENTALS) \
	additional-drivers \
	$(DOCUMENTATION) _output/share/doc/lima/templates \
	_output/share/man/man1/limactl.1

# Internal: create tar.gz artifact bundle
$(ARTIFACT_PATH_COMMON).tar.gz: $(ARTIFACT_DES) | _artifacts
	$(TAR) -C _output/ --no-xattrs -czvf $@ ./

# Internal: create tar.gz artifact for additional guest agents
$(ARTIFACT_ADDITIONAL_GUESTAGENTS_PATH_COMMON).tar.gz:
	# FIXME: do not exec make from make
	make clean additional-guestagents
	$(TAR) -C _output/ --no-xattrs -czvf $@ ./

# Internal: create zip artifact bundle (Windows)
$(ARTIFACT_PATH_COMMON).zip: $(ARTIFACT_DES) | _artifacts
	cd _output && $(ZIP) -r ../$@ *

# Internal: create zip artifact for additional guest agents (Windows)
$(ARTIFACT_ADDITIONAL_GUESTAGENTS_PATH_COMMON).zip:
	make clean additional-guestagents
	cd _output && $(ZIP) -r ../$@ *

# Generate manpages using native limactl regardless of target GOOS
manpages-using-native-limactl: GOOS = $(GOHOSTOS)
manpages-using-native-limactl: GOARCH = $(GOHOSTARCH)
manpages-using-native-limactl: manpages

# returns "manpages-using-native-limactl" if $(1) is not equal to $(GOHOSTOS).
# $(1): GOOS
generate_manpages_if_needed = $(if $(filter $(if $(1),$(1),$(GOOS)),$(GOHOSTOS)),,manpages-using-native-limactl)

# build native arch first, then build other archs.
artifact_goarchs = arm64 amd64
goarchs_native_and_others = $(GOHOSTARCH) $(filter-out $(GOHOSTARCH),$(artifact_goarchs))

# artifacts is artifact bundles for each OS.
# if target GOOS is native, build native arch first, generate manpages, then build other archs.
# if target GOOS is not native, build native limactl, generate manpages, then build the target GOOS with archs.
.PHONY: artifacts artifacts-darwin artifacts-linux artifacts-windows

# Build artifacts for the current GOOS across all supported architectures
artifacts: $$(addprefix artifact-$$(GOOS)-,$$(goarchs_native_and_others))

# Build macOS (darwin) artifacts for all supported architectures
artifacts-darwin: $$(call generate_manpages_if_needed,darwin) $$(addprefix artifact-darwin-,$$(goarchs_native_and_others))

# Build Linux artifacts for all supported architectures
artifacts-linux: $$(call generate_manpages_if_needed,linux) $$(addprefix artifact-linux-,$$(goarchs_native_and_others))

# Build Windows artifacts for all supported architectures
artifacts-windows: $$(call generate_manpages_if_needed,windows) $$(addprefix artifact-windows-,$$(goarchs_native_and_others))

# set variables for artifact variant targets.
artifact-darwin% artifact-darwin: GOOS = darwin
artifact-linux% artifact-linux: GOOS = linux
artifact-windows% artifact-windows: GOOS = windows
artifact-%-amd64 artifact-%-x86_64 artifact-amd64 artifact-x86_64: GOARCH = amd64
artifact-%-arm64 artifact-%-aarch64 artifact-arm64 artifact-aarch64: GOARCH = arm64

# build cross arch binaries.
artifact-%: $$(call generate_manpages_if_needed)
	make clean artifact GOOS=$(GOOS) GOARCH=$(GOARCH)

.PHONY: artifacts-misc
artifacts-misc: | _artifacts
	go mod vendor
	$(TAR) --no-xattrs -czf _artifacts/lima-$(VERSION_TRIMMED)-go-mod-vendor.tar.gz go.mod go.sum vendor

MKDIR_TARGETS += _artifacts

################################################################################
# This target must be placed after any changes to the `MKDIR_TARGETS` variable.
# It seems that variable expansion in Makefile targets is not done recursively.
$(MKDIR_TARGETS):
	mkdir -p $@
