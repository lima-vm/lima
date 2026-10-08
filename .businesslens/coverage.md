---
scope: The limactl command line and its wrappers, the MCP plugin, and the instance, host agent, template, disk, network and download code behind them.
method: Static reading of source, shipped templates and documentation; nothing was built or run.
covered:
  - description: limactl commands for instances, snapshots, disks, networks, templates, automatic startup and diagnostics.
    paths:
      - cmd/limactl/
  - description: The lima shell wrapper and the container and Kubernetes command wrappers.
    paths:
      - cmd/lima
      - cmd/lima.bat
      - cmd/nerdctl.lima
      - cmd/docker.lima
      - cmd/podman.lima
      - cmd/kubectl.lima
      - cmd/apptainer.lima
  - description: The MCP plugin and its tool set.
    paths:
      - cmd/limactl-mcp/
      - pkg/mcp/
  - description: Instance lifecycle, inspection and on-disk store.
    paths:
      - pkg/instance/
      - pkg/store/
      - pkg/limatype/
  - description: Template locators, base-template embedding and the template store.
    paths:
      - pkg/limatmpl/
      - pkg/templatestore/
  - description: Configuration defaults, merging and validation.
    paths:
      - pkg/limayaml/
  - description: Restricted yq expression evaluation.
    paths:
      - pkg/yqutil/
  - description: Host agent start-up, readiness requirements, events and port forwarding.
    paths:
      - pkg/hostagent/
      - pkg/portfwd/
  - description: Snapshot dispatch to drivers.
    paths:
      - pkg/snapshot/
  - description: Lima disk image creation and resizing.
    paths:
      - pkg/imgutil/
      - pkg/qemuimgutil/
  - description: Lima network configuration.
    paths:
      - pkg/networks/
  - description: Automatic startup registration.
    paths:
      - pkg/autostart/
  - description: Download cache and digest checking.
    paths:
      - pkg/downloader/
  - description: Plugin discovery.
    paths:
      - pkg/plugins/
  - description: File copy backends.
    paths:
      - pkg/copytool/
  - description: Diagnostic information.
    paths:
      - pkg/limainfo/
  - description: Host block device path validation.
    paths:
      - pkg/blockdevice/
exclusions:
  - description: Project website and documentation sources.
    paths:
      - website/
      - docs/
  - description: Test suites, CI and release tooling.
    paths:
      - hack/
      - .github/
      - Makefile
  - description: Packaging commands that generate manual pages and the JSON schema.
    paths:
      - cmd/limactl/gendoc.go
      - cmd/limactl/genschema.go
  - description: Shell completion scripts.
    paths:
      - cmd/limactl/completion.go
      - cmd/limactl/tab_completion.go
  - description: Generated protobuf bindings.
    paths:
      - pkg/driver/external/driver.pb.go
      - pkg/driver/external/driver_grpc.pb.go
      - pkg/guestagent/api/guestservice.pb.go
unmapped:
  - description: 'Virtualization drivers beyond the snapshot, screenshot and status behavior modeled: display, audio, GPU, Rosetta, nested virtualization, TPM, block-device attachment and driver-specific options.'
    paths:
      - pkg/driver/
      - cmd/lima-driver-qemu/
      - cmd/lima-driver-vz/
      - cmd/lima-driver-wsl2/
      - cmd/lima-driver-krunkit/
      - pkg/registry/
  - description: Guest boot scripts and cloud-init data that apply provisioning, mounts, disks and containerd inside the guest.
    paths:
      - pkg/cidata/
  - description: The guest agent's port and Kubernetes service discovery.
    paths:
      - pkg/guestagent/
      - cmd/lima-guestagent/
  - description: The user-v2 network daemon, the built-in DNS and the privileged network and block-device helpers.
    paths:
      - pkg/networks/usernet/
      - pkg/hostagent/dns/
      - cmd/lima-privileged-net/
      - cmd/lima-privileged-block-device/
  - description: 'Hidden limactl commands: guest-install, debug, usernet and hostagent.'
    paths:
      - cmd/limactl/guest-install.go
      - cmd/limactl/debug.go
      - cmd/limactl/usernet.go
      - cmd/limactl/hostagent.go
  - description: The fedora-rawhide URL-scheme plugin.
    paths:
      - cmd/limactl-url-fedora-rawhide
  - description: What each shipped template provisions in its guest.
    paths:
      - templates/
limitations:
  - description: WSL2-only instance statuses Uninitialized and Installing, whose transitions were not established.
    paths:
      - pkg/driver/wsl2/
---

# Coverage
