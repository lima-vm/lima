# Internal data structure

## Instance directory (`~/.lima/<INSTANCE>`)

An instance directory contains the following files:

Metadata:
- `lima.yaml`: the YAML

cloud-init:
- `cidata.iso`: cloud-init ISO9660 image. (`user-data`, `meta-data`, `lima-guestagent.Linux-<ARCH>`)

disk:
- `basedisk`: the base image
- `diffdisk`: the diff image (QCOW2)

QEMU:
- `qemu.pid`: QEMU PID
- `qmp.sock`: QMP socket
- `serial.log`: QEMU serial log, for debugging
- `serial.sock`: QEMU serial socket, for debugging (Usage: `socat -,echo=0,icanon=0 unix-connect:serial.sock`)

SSH:
- `ssh.sock`: SSH control master socket

Guest agent:
- `ga.sock`: Forwarded to `/run/user/$UID/lima-guestagent.sock` in the guest, via SSH

Host agent:
- `ha.pid`: hostagent PID
- `ha.stdout.log`: hostagent stdout (JSON lines, see `pkg/hostagent/api.Events`)
- `ha.stderr.log`: hostagent stderr (human-readable messages)

## Cache directory (`~/Library/Caches/lima/download/by-url-sha256/<SHA256_OF_URL>`)

The directory contains the following files:

- `url`: raw url text, without "\n"
- `data`: data
