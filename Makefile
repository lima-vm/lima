# Files are installed under $(DESTDIR)/$(PREFIX)
PREFIX ?= /usr/local
DEST := $(shell echo "$(DESTDIR)/$(PREFIX)" | sed 's:///*:/:g; s://*$$::')

GO ?= go

TAR ?= tar

PACKAGE := github.com/lima-vm/lima

VERSION=$(shell git describe --match 'v[0-9]*' --dirty='.m' --always --tags)
VERSION_TRIMMED := $(VERSION:v%=%)

GO_BUILD := CGO_ENABLED=0 $(GO) build -ldflags="-s -w -X $(PACKAGE)/pkg/version.Version=$(VERSION)"

.PHONY: all
all: binaries

.PHONY: binaries
binaries: \
	_output/bin/lima \
	_output/bin/limactl \
	_output/bin/nerdctl.lima \
	_output/share/lima/lima-guestagent.Linux-x86_64.gz \
	_output/share/lima/lima-guestagent.Linux-aarch64.gz
	mkdir -p _output/share/doc/lima
	cp -aL README.md LICENSE docs examples _output/share/doc/lima
	echo $(VERSION) > _output/share/doc/lima/VERSION

.PHONY: _output/bin/lima
_output/bin/lima:
	mkdir -p _output/bin
	cp -a ./cmd/lima $@

.PHONY: _output/bin/nerdctl.lima
_output/bin/nerdctl.lima:
	mkdir -p _output/bin
	cp -a ./cmd/nerdctl.lima $@

.PHONY: _output/bin/limactl
_output/bin/limactl:
	$(GO_BUILD) -o $@ ./cmd/limactl

.PHONY: _output/share/lima/lima-guestagent.Linux-x86_64.gz
_output/share/lima/lima-guestagent.Linux-x86_64.gz:
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -o $(basename $@) ./cmd/lima-guestagent
	# Compress binary to let Homebrew ignore "Binaries built for a non-native architecture" check.
	gzip -1fn $(basename $@)

.PHONY: _output/share/lima/lima-guestagent.Linux-aarch64.gz
_output/share/lima/lima-guestagent.Linux-aarch64.gz:
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -o $(basename $@) ./cmd/lima-guestagent
	# Compress binary to let Homebrew ignore "Binaries built for a non-native architecture" check.
	gzip -1fn $(basename $@)

.PHONY: install
install:
	mkdir -p "$(DEST)"
	cp -av _output/* "$(DEST)"
	if [[ $(shell uname -s ) != Linux && ! -e "$(DEST)/bin/nerdctl" ]]; then ln -sf nerdctl.lima "$(DEST)/bin/nerdctl"; fi

.PHONY: uninstall
uninstall:
	@test -f "$(DEST)/bin/lima" || (echo "lima not found in $(DEST) prefix"; exit 1)
	rm -rf \
		"$(DEST)/bin/lima" \
		"$(DEST)/bin/limactl" \
		"$(DEST)/bin/nerdctl.lima" \
		"$(DEST)/share/lima" "$(DEST)/share/doc/lima"
	# TODO: remove $(DEST)/bin/nerdctl only when it is a symlink to nerdctl.lima

.PHONY: clean
clean:
	rm -rf _output

.PHONY: artifacts
artifacts:
	mkdir -p _artifacts
	GOOS=darwin GOARCH=amd64 make clean binaries
	$(TAR) -C _output/ -czvf _artifacts/lima-$(VERSION_TRIMMED)-Darwin-x86_64.tar.gz ./
	GOOS=darwin GOARCH=arm64 make clean binaries
	$(TAR) -C _output -czvf _artifacts/lima-$(VERSION_TRIMMED)-Darwin-arm64.tar.gz ./
	GOOS=linux GOARCH=amd64 make clean binaries
	$(TAR) -C _output/ -czvf _artifacts/lima-$(VERSION_TRIMMED)-Linux-x86_64.tar.gz ./
	GOOS=linux GOARCH=arm64 make clean binaries
	$(TAR) -C _output/ -czvf _artifacts/lima-$(VERSION_TRIMMED)-Linux-aarch64.tar.gz ./
