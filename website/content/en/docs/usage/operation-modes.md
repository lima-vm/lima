---
title: Lima Operation Modes
weight: 2
---

Lima operates in different modes that determine how it integrates with your host system and what features are available. Understanding these modes helps you choose the right configuration for your use case and explains some behaviors that might seem surprising at first.

## Overview

Lima provides two primary operation modes:

- **Integrated Mode (Default)**: Provides seamless host-guest integration with automatic mounts, port forwarding, and container engines
- **Plain Mode**: Offers a traditional VM experience with minimal host integration

This document explains both modes in detail to help you understand Lima's default behavior and choose the right mode for your needs.

## Integrated Mode (Default)

By default, Lima operates in "integrated mode," which is designed to provide seamless integration between the host and guest systems. This mode prioritizes convenience and developer productivity by automatically setting up various integrations.

### Key Features of Integrated Mode

#### 1. User Mirroring
Lima automatically mirrors your host user information into the guest VM:

- **Username**: Your host username is copied to the guest (with fallback to "lima" if invalid)
- **User ID (UID)**: Your host UID is preserved in the guest for consistent file ownership
- **User Comment/Full Name**: Your host user's full name is copied
- **Home Directory**: A Linux-compatible home directory is created (typically `/home/username.linux`)

This ensures that files you create in the guest have the correct ownership when viewed from the host.

#### 2. Automatic Directory Mounting
Lima automatically mounts key directories from your host:

- **Home Directory Mount**: Your entire home directory (`~`) is mounted read-only at the same path in the guest
- **Shared Workspace**: `/tmp/lima` is mounted as a writable shared space between host and guest

These mounts allow you to:
- Access your host files directly from within the VM
- Share files between host and guest without manual copying
- Maintain a consistent development environment

#### 3. Port Forwarding
Lima automatically forwards ports from the guest to the host:

- Published ports in containers are automatically forwarded to `localhost` on the host
- SSH access is available on a local port (typically 60022 for the "default" instance)
- Network services running in the guest become accessible from the host

#### 4. Container Engine Integration
Lima can automatically install and configure container engines:

- **containerd**: Installed by default in user mode (rootless)
- **Docker/Podman**: Available through templates
- **Kubernetes**: Can be configured through templates like k3s, k8s

#### 5. Guest Agent Services
A guest agent runs inside the VM to:
- Manage port forwarding
- Handle file system mounts
- Coordinate host-guest integration
- Provide status information

### When to Use Integrated Mode

Integrated mode is ideal for:
- **Container Development**: When you need Docker/Podman/containerd with host integration
- **Development Workflows**: When you want to edit files on the host but run them in Linux
- **Cross-platform Development**: When developing Linux applications on macOS/Windows
- **CI/CD Testing**: When you need a Linux environment that integrates with host tools

### Configuration Example

```yaml
# Integrated mode is the default - these settings are applied automatically
user:
  # Host user information is automatically mirrored
  name: null  # Will use your host username
  uid: null   # Will use your host UID

mounts:
  # Home directory mounted read-only
  - location: "~"
    writable: false
  # Shared writable space
  - location: "/tmp/lima"
    writable: true

containerd:
  user: true  # Rootless containerd enabled by default

# Port forwarding enabled
# Guest agent enabled
# Host integration enabled
```

## Plain Mode

Plain mode disables Lima's host integration features, providing a more traditional VM experience similar to what users might expect from Vagrant or basic virtualization tools.

### Key Features of Plain Mode

#### 1. No Automatic Mounts
- No home directory mounting
- No shared directories
- You must explicitly configure any mounts you need

#### 2. No Port Forwarding
- Ports are not automatically forwarded from guest to host
- You must manually configure any port forwarding rules
- Network access requires explicit configuration

#### 3. No Container Engine Integration
- containerd is not automatically installed or configured
- No automatic container runtime setup
- You must manually install and configure container engines

#### 4. No Guest Agent
- The Lima guest agent does not run
- No automatic host-guest coordination
- Reduced background processes in the guest

#### 5. Minimal Host Integration
- User information may still be mirrored for SSH access
- Basic VM lifecycle management still works
- SSH access is still available

### When to Use Plain Mode

Plain mode is ideal for:
- **Security-Conscious Environments**: When you want minimal host exposure
- **Traditional VM Usage**: When you prefer explicit control over all integrations
- **Learning/Educational**: When you want to understand Linux systems without abstractions
- **Custom Setups**: When you need full control over the guest environment
- **Isolation**: When you want the guest to be completely separate from the host

### Enabling Plain Mode

To enable plain mode, set the `plain` field to `true` in your Lima configuration:

```yaml
# Enable plain mode
plain: true

# In plain mode, these are automatically disabled:
# - mounts: []
# - portForwards: []
# - containerd.system: false
# - containerd.user: false
# - Guest agent services
```

You can also enable plain mode when creating an instance:

```bash
limactl create --plain my-plain-vm
```

## Comparison: Integrated Mode vs Plain Mode

| Feature | Integrated Mode | Plain Mode |
|---------|--------------|------------|
| **Home Directory Mount** | ✅ Automatic (`~` read-only) | ❌ None |
| **Shared Directory** | ✅ `/tmp/lima` writable | ❌ None |
| **Port Forwarding** | ✅ Automatic | ❌ None |
| **Container Engine** | ✅ containerd by default | ❌ None |
| **Guest Agent** | ✅ Running | ❌ Disabled |
| **User Mirroring** | ✅ Full mirroring | ✅ Basic (for SSH) |
| **Host Integration** | ✅ Seamless | ❌ Minimal |
| **Security** | ⚠️ More host exposure | ✅ More isolated |
| **Convenience** | ✅ High | ⚠️ Manual setup required |

## Understanding the Default Behavior

### Why Does Lima Auto-Mount the Home Directory?

Lima's default behavior of mounting the home directory is designed to provide seamless integration similar to Docker Machine or Podman Machine. This design choice:

1. **Enables Seamless Development**: You can edit files on your host with your preferred tools and run them in the Linux environment
2. **Maintains File Ownership**: The UID mirroring ensures files have correct permissions
3. **Reduces Context Switching**: No need to copy files back and forth between host and guest
4. **Supports Container Workflows**: Container images can access host files for development

### Common User Expectations

**Users from Docker Machine/Podman Machine Background** typically expect:
- Automatic host integration
- Seamless file access
- Port forwarding
- Container engine ready to use

**Users from Vagrant Background** might expect:
- Isolated VM environment
- Manual configuration of shared folders
- Explicit port forwarding setup
- No automatic services

### Security Considerations

#### Integrated Mode Security Implications
- **Host File Exposure**: Your entire home directory is accessible from the guest
- **Network Exposure**: Ports are automatically forwarded
- **Process Visibility**: Guest agent runs with host integration

#### Plain Mode Security Benefits
- **Isolation**: Guest has minimal access to host resources
- **Explicit Control**: All integrations must be manually configured
- **Reduced Attack Surface**: Fewer automatic services and mounts

## Migration Between Modes

### From Integrated Mode to Plain Mode

If you want to disable the automatic integrations:

```yaml
# Disable integrated mode features
plain: true

# Or selectively disable features:
plain: false
mounts: []  # No automatic mounts
containerd:
  user: false
  system: false
portForwards: []  # No automatic port forwarding
```

### From Plain Mode to Integrated Mode

To enable integrated mode features in a plain mode instance:

```yaml
plain: false

# Re-enable default mounts
mounts:
  - location: "~"
  - location: "/tmp/lima"
    writable: true

# Re-enable containerd
containerd:
  user: true

# Port forwarding will be automatic
```

## Best Practices

### For Integrated Mode
- **Review Mounted Directories**: Be aware of what host directories are accessible
- **Use Read-Only Mounts**: Keep the home directory read-only unless you need to write
- **Monitor Port Forwarding**: Be aware of which ports are being forwarded
- **Regular Updates**: Keep the guest agent and container engines updated

### For Plain Mode
- **Explicit Configuration**: Document all manual configurations you make
- **Security Review**: Regularly audit what access the VM has to host resources
- **Backup Strategy**: Plan for data persistence since there are no automatic mounts
- **Network Planning**: Design your network access and port forwarding explicitly

### For Both Modes
- **Resource Monitoring**: Monitor CPU, memory, and disk usage
- **Regular Maintenance**: Keep the guest OS and packages updated
- **Configuration Management**: Version control your Lima YAML configurations
- **Testing**: Test your setup in both development and production-like scenarios

## Troubleshooting Common Issues

### "Why is my home directory mounted?"
This is the default behavior in integrated mode. If you don't want this:
- Use plain mode (`plain: true`)
- Or remove the home mount: `mounts: []`

### "Why can't I access my files?"
In plain mode, no directories are automatically mounted. You need to:
- Add explicit mounts in your configuration
- Or copy files manually using `limactl copy`

### "Ports aren't being forwarded"
In plain mode, port forwarding is disabled. You need to:
- Add explicit `portForwards` configuration
- Or use plain networking and access the VM's IP directly

### "Container engine isn't available"
In plain mode, container engines aren't automatically installed:
- Install manually: `sudo apt install docker.io` (or similar)
- Or disable plain mode to get automatic containerd

## Related Documentation

- [Configuration Guide](../config/) - Detailed configuration options
- [Filesystem Mounts](../config/mount) - File system mounting options
- [Network Configuration](../config/network/) - Network and port forwarding
- [Port Configuration](../config/port) - Port forwarding configuration
- [Templates](../templates/) - Pre-configured Lima templates
- [FAQ](../faq/) - Frequently asked questions

## Summary

Lima's operation modes provide flexibility for different use cases:

- **Integrated Mode (Default)**: Best for development workflows with seamless host-guest integration
- **Plain Mode**: Best for traditional VM usage with explicit control and better isolation

Choose the mode that best fits your security requirements, workflow preferences, and integration needs. You can always switch between modes by modifying your Lima configuration file.