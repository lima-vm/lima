# Internal data structure

## Lima home directory (`${LIMA_HOME}`)

Defaults to `~/.lima`.

Note that we intentionally avoid using `~/Library/Application Support/Lima` on macOS.

We use `~/.lima` so that we can have enough space for the length of the socket path,
which must be less than 104 characters on macOS.

### Config directory (`${LIMA_HOME}/_config`)

The config directory contains global lima settings that apply to all instances.

User identity:

Lima creates a default identity and uses its public key as the authorized key
to access all lima instances. In addition lima will also configure all public
keys from `~/.ssh/*.pub` as well, so the user can use the ssh endpoint without
having to specify an identity explicitly.
- `user`: private key
- `user.pub`: public key

### Instance directory (`${LIMA_HOME}/<INSTANCE>`)

An instance directory contains the following files:

Metadata:
- `lima.yaml`: the YAML

cloud-init:
- `cidata.iso`: cloud-init ISO9660 image. See [`cidata.iso`](#cidataiso).

disk:
- `basedisk`: the base image
- `diffdisk`: the diff image (QCOW2)

QEMU:
- `qemu.pid`: QEMU PID
- `qmp.sock`: QMP socket
- `serial.log`: QEMU serial log, for debugging
- `serial.sock`: QEMU serial socket, for debugging (Usage: `socat -,echo=0,icanon=0 unix-connect:serial.sock`)

Samba:
- `samba.tmp/credentials`: credentials for `mount -t cifs -o credentials=<CREDENTIALS>`. Passed to the guest via `qemu_fw_cfg`.
- `samba.tmp/smb.conf`: smb.conf
- `samba.tmp/state`: samba state dir, including `smbpasswd`

SSH:
- `ssh.sock`: SSH control master socket

Guest agent:
- `ga.sock`: Forwarded to `/run/lima-guestagent.sock` in the guest, via SSH

Host agent:
- `ha.pid`: hostagent PID
- `ha.stdout.log`: hostagent stdout (JSON lines, see `pkg/hostagent/api.Events`)
- `ha.stderr.log`: hostagent stderr (human-readable messages)

## Lima cache directory (`~/Library/Caches/lima`)

Currently hard-coded to `~/Library/Caches/lima` on macOS.

### Download cache (`~/Library/Caches/lima/download/by-url-sha256/<SHA256_OF_URL>`)

The directory contains the following files:

- `url`: raw url text, without "\n"
- `data`: data
- `<ALGO>.digest`: digest of the data, in OCI format.
   e.g., file name `sha256.digest`, with content `sha256:5ba3d476707d510fe3ca3928e9cda5d0b4ce527d42b343404c92d563f82ba967`

## Environment variables

- `$LIMA_HOME`: The "Lima home directory" (see above).
  - Default : `~/.lima`

- `$LIMA_INSTANCE`: `lima ...` is expanded to `limactl shell ${LIMA_INSTANCE} ...`.
  - Default : `default`

- `$QEMU_SYSTEM_X86_64`: path of `qemu-system-x86_64`
  - Default: `qemu-system-x86_64` in `$PATH`

- `$QEMU_SYSTEM_AARCH64`: path of `qemu-system-aarch64`
  - Default: `qemu-system-aarch64` in `$PATH`

## `cidata.iso`
`cidata.iso` contains the following files:

- `user-data`: [Cloud-init user-data](https://cloudinit.readthedocs.io/en/latest/topics/format.html)
- `meta-data`: [Cloud-init meta-data](https://cloudinit.readthedocs.io/en/latest/topics/instancedata.html)
- `network-config`: [Cloud-init Networking Config Version 2](https://cloudinit.readthedocs.io/en/latest/topics/network-config-format-v2.html)
- `lima.env`: The `LIMA_CIDATA_*` environment variables (see below) available during `boot.sh` processing
- `lima-guestagent`: Lima guest agent binary
- `nerdctl-full.tgz`: [`nerdctl-full-<VERSION>-linux-<ARCH>.tar.gz`](https://github.com/containerd/nerdctl/releases)
- `boot.sh`: Boot script
- `boot/*`: Boot script modules
- `provision.system/*`: Custom provision scripts (system)
- `provision.user/*`: Custom provision scripts (user)
- `etc_environment`: Environment variables to be added to `/etc/environment` (also loaded during `boot.sh`)

Max file name length = 30

Secret files are provided to the guest via `qemu_fw_cfg`, not via this virtual CD-ROM.

### Volume label
The volume label is "cidata", as defined by [cloud-init NoCloud](https://cloudinit.readthedocs.io/en/latest/topics/datasources/nocloud.html).

### Environment variables
- `LIMA_CIDATA_MNT`: the mount point of the disk. `/mnt/lima-cidata`.
- `LIMA_CIDATA_USER`: the user name string
- `LIMA_CIDATA_UID`: the numeric UID
- `LIMA_CIDATA_MOUNTS`: the number of the Lima mounts
- `LIMA_CIDATA_MOUNTS_%d_MOUNTPOINT`: the N-th mount point of Lima mounts (N=0, 1, ...)
- `LIMA_CIDATA_MOUNTS_%d_WRITABLE`: set to "1" if the N-th mount point is writable
- `LIMA_CIDATA_CONTAINERD_USER`: set to "1" if rootless containerd to be set up
- `LIMA_CIDATA_CONTAINERD_SYSTEM`: set to "1" if system-wide containerd to be set up
- `LIMA_CIDATA_SLIRP_GATEWAY`: set to the IP address of the host on the SLIRP network. `192.168.5.2`.
- `LIMA_CIDATA_SLIRP_FILESERVER`: set to the IP address of the virtual file server (i.e., Samba). `192.168.5.4`.

## `qemu_fw_cfg` (used for Secrets)

- `opt/io.github.lima-vm.lima.samba-credentials`: Samba credentials

- - -

# File sharing

Starting with Lima v0.7.0, file sharing is implemented using Samba-over-Stdio.

Previous versions were using Reverse SSHFS, but dropped due to poor performance and instability.

Samba (default: 192.168.5.4) is accessible from the guest via TCP/IP, but not accessible from the host,
because the `smbd` process is connected to QEMU via the stdio of the `smbd` process.

Currently, Lima uses legacy SMB1 protocol to support UNIX extensions, because Samba still does not support
UNIX extensions for SMB3.1.1 (See FAQs below).

The `smbpasswd` is randomly regenerated on every boot of the guest.

## FAQs
### Why use stdio?
For hiding the Samba port from the host processes, such as malicious WebSocket apps that can
connect to arbitrary IP including 127.0.0.1.

Run `sudo lsof -i  -P | grep LISTEN` to confirm that no real TCP port on the host is listening for Samba.

### Why use deprecated SMB1?

Because Samba still does not support UNIX extensions for SMB3.1.1 (https://bugs.launchpad.net/ubuntu/+source/samba/+bug/1883234).

[The release note of Samba 4.11 (Sep 2019)](https://wiki.samba.org/index.php/Samba_4.11_Features_added/changed#SMB1_is_disabled_by_default)
says `SMB1 is officially deprecated and might be removed step by step in the following years.`,
so, we should switch away from SMB1, as soon as Samba supports UNIX extensions for SMB3.1.1.

Note that Lima configures Samba to be accessible only from the QEMU process via the stdio of the `smbd` process,
so the security risk of SMB1 is minimized in our case.

### Why not NFS?
Because authentication is not easy.

The common authentication method is to just check whether the src port is a privileged port number (1-1023), but it
does not work well with the stdio technique explained above, as the server process cannot detect the port number.

TODO: consider implementing a usermode NFS server, perhaps using [go-nfs](https://github.com/willscott/go-nfs) when it matured enough.
Maybe we could use Ganesha NFS, but it doesn't seem to support using stdio nor UNIX sockets (and Ganesha is not available in Homebrew).
Auth should be implemented using krb5(i), but just running `iptables` to prohibit unprivileged processes from accessing the NFS port might be enough.

### Why not 9P?
- virtio-9p-pci is unimplemented for macOS
- 9P over virtserial is slow
- 9P over TCP is insecure, and authentication is not easy (TODO: consider using `iptables` to prohibit unprivileged processes from accessing the 9P port)
