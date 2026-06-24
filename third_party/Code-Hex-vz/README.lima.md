# Vendored fork of github.com/Code-Hex/vz/v3 (v3.7.1)

This is a temporary in-tree fork carrying one addition needed for VZ runtime
hot-mount: `(*VirtualMachine).SetVirtioFileSystemDeviceShareAtIndex`, which sets
the directory share of a live virtio-fs device on a running VM.

The isolated change is also captured in `hack/patches/code-hex-vz-runtime-share.patch`
and should be upstreamed to Code-Hex/vz; once released, drop this fork and the
`replace` directive in go.mod and bump the dependency.
