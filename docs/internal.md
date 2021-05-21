# Internal data structure

## Instance directory (`~/.lima/<INSTANCE>`)

An instance directory contains the following files:

- `lima.yaml`: the YAML
- `cidata.iso`: cloud-init ISO9660 image. (`user-data`, `meta-data`, `lima-guestagent.Linux-<ARCH>`)
- `basedisk`: the base image
- `diffdisk`: the diff image (QCOW2)
- `qemu-pid`: PID of the QEMU
- `ssh.sock`: SSH control master socket
- `ga.sock`: Forwarded to `/run/user/$UID/lima-guestagent.sock`
- `serial.log`: QEMU serial log, for debugging
- `serial.sock`: QEMU serial socket, for debugging (Usage: `socat -,echo=0,icanon=0 unix-connect:serial.sock`)
