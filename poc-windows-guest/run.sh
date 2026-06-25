#!/bin/bash

# Note that this script expects that you've already set Windows OS ISO (windows_server_2025.iso) and virtio-win ISO (virtio-win-0.1.285.iso) in ./images/

set -e

# Cleanup
rm -f images/windows.qcow2
rm -f images/win-cidata.iso

# Build win-cidata.iso
mkdir -p tmp/win-cidata
cp autounattend.xml ./tmp/win-cidata/
cp first_logon.ps1 ./tmp/win-cidata/
cp ~/.lima/_config/user.pub ./tmp/win-cidata/
mkisofs -o ./images/win-cidata.iso -J -r -V "autoanattend" ./tmp/win-cidata/

# Create qcow2
qemu-img create -f qcow2 ./images/windows.qcow2 40G

# Run virtiofsd in background
virtiofsd --shared-dir=./ --socket-path=./virtiofsd.sock --sandbox none &

# Run QEMU
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
    -device vhost-user-fs-pci,chardev=chr-vu-fs0,tag=lima \
    -device virtio-rng-pci
