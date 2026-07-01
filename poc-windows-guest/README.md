## PoC: Windows guest VM with VirtIO-FS filesystem mount
This PoC creates a Windows Server 2025 guest VM using QEMU. This PoC supports:
- Fully automated installation via autounattend
- Install VirtIO drivers for performance improvements, and VirtIO-FS
- Filesystem mounts through VirtIO-FS
- SSH from host computer with a public key

### Files
- autounattend.xml
  - This automates OS installation
- first_logon.ps1
  - It installs/sets up various tools on Windows guest. It is called through autounattend.xml.
- images/
  - This directory contains ISO and qcow2 files

### Host environment
I tested the code on:
CPU arch: x86_64
OS: Ubuntu 24.04

### Prerequisites
You should have:
- QEMU binary
- mkisofs (or altanatives) for creating ISO file
  - Through apt, you can install it via `apt install genisoimage`
- virtiofsd binary
  - You can install via `cargo install virtiofsd`

### Prepare ISO files
#### Download Windows Server 2025 ISO
You can download the ISO file from https://www.microsoft.com/en-us/evalcenter/evaluate-windows-server-2025.
Note that you need to register your name, email, company, and so on.
After downloading it, you should place the ISO file as `poc-windows-guest/images/windows_server_2025.iso`.

#### Download VirtIO-Win ISO
You can download the ISO file from https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/archive-virtio/virtio-win-0.1.285-1/.
After downloading it, please place the ISO file under `poc-windows-guest/images/`. You don't need to rename it.

### Launch Windows VM
You can also use poc-windows-guest/run.sh for executing those commands.
Please move to `./poc-windows-guest` before executing those commands.

#### Create autounattend ISO file
The ISO file contains autounattend.xml, first_logon.ps1, and a public key.
```bash
mkdir -p tmp/win-cidata
cp autounattend.xml ./tmp/win-cidata/
cp first_logon.ps1 ./tmp/win-cidata/
cp ~/.lima/_config/user.pub ./tmp/win-cidata/
mkisofs -o ./images/win-cidata.iso -J -r -V "autoanattend" ./tmp/win-cidata/
```

#### Run virtiofsd
```bash
virtiofsd --shared-dir=./ --socket-path=./virtiofsd.sock --sandbox none &
```

#### Create qcow2 file
```bash
qemu-img create -f qcow2 ./images/windows.qcow2 40G
```

#### Run QEMU
```bash
qemu-system-x86_64 \
    -name guest=win2k25,debug-threads=on \
    -machine pc-q35-noble,usb=off,vmport=off,dump-guest-core=off,memory-backend=pc.ram,hpet=off,acpi=on \
    -accel kvm \
    -cpu host,migratable=on,hv-time=on,hv-relaxed=on,hv-vapic=on,hv-spinlocks=0x1fff \
    -m size=2048 \
    -object memory-backend-memfd,id=pc.ram,share=true,size=2048M \
    -overcommit mem-lock=off \
    -smp 2,sockets=2,cores=1,threads=1 \
    -drive file=./images/windows.qcow2,if=virtio,id=disk0,discard=on \
    -blockdev driver=file,filename=./images/windows_server_2025.iso,node-name=cdrom0-storage,read-only=true \
    -blockdev driver=raw,file=cdrom0-storage,node-name=cdrom0,read-only=true \
    -device ide-cd,bus=ide.1,drive=cdrom0 \
    -blockdev driver=file,filename=./images/virtio-win-0.1.285.iso,node-name=cdrom1-storage,read-only=true \
    -blockdev driver=raw,file=cdrom1-storage,node-name=cdrom1,read-only=true \
    -device ide-cd,bus=ide.2,drive=cdrom1 \
    -blockdev driver=file,filename=./images/win-cidata.iso,node-name=cdrom2-storage,read-only=true \
    -blockdev driver=raw,file=cdrom2-storage,node-name=cdrom2,read-only=true \
    -device ide-cd,bus=ide.3,drive=cdrom2 \
    -netdev user,id=net0,net=192.168.10.0/24,dhcpstart=192.168.10.15,hostfwd=tcp:127.0.0.1:60022-:22 \
    -device virtio-net-pci,netdev=net0 \
    -chardev socket,id=chr-vu-fs0,path=./virtiofsd.sock \
    -device vhost-user-fs-pci,chardev=chr-vu-fs0,tag=hoge \
    -device virtio-rng-pci
```

#### Try SSH
```bash
ssh -p 60022 limauser@localhost -i ~/.lima/_config/user
Microsoft Windows [Version 10.0.26100.32230]
(c) Microsoft Corporation. All rights reserved.

limauser@LIMA C:\Users\limauser>
```

#### Check shared filesystem
By default, VirtIO-FS mounts a shared filesystem on `Z:`
```bash
limauser@LIMA C:\Users\limauser>dir Z:\
 Volume in drive Z is lima
 Volume Serial Number is 0000-0000

 Directory of Z:\

01/01/1970  01:00 AM    <DIR>          .
01/01/1970  01:00 AM    <DIR>          ..
05/17/2026  10:17 AM             3,652 README.md
05/17/2026  10:01 AM             7,363 autounattend.xml
05/17/2026  10:01 AM             2,014 first_logon.ps1
05/17/2026  10:05 AM    <DIR>          images
05/17/2026  10:00 AM             2,153 run.sh
05/17/2026  10:05 AM    <DIR>          tmp
05/17/2026  10:05 AM                 6 virtiofsd.sock.pid
               5 File(s)         23,380 bytes
               4 Dir(s)  29,202,513,920 bytes free
```
