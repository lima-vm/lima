#!/usr/bin/env bash
set -euo pipefail

RED='\033[0;31m'
GRN='\033[0;32m'
YLW='\033[1;33m'
RST='\033[0m'

info()  { echo -e "${GRN}==>${RST} $*"; }
warn()  { echo -e "${YLW}  ! $*${RST}"; }
fatal() { echo -e "${RED}ERR${RST} $*" >&2; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGES_DIR="$SCRIPT_DIR/images"
TMP_DIR="$SCRIPT_DIR/tmp/win-cidata"
QCOW2="$IMAGES_DIR/windows.qcow2"
CIDATA_ISO="$IMAGES_DIR/win-cidata.iso"
WIN_ISO="$IMAGES_DIR/windows_server_2025.iso"
VIRTIO_ISO="$IMAGES_DIR/virtio-win-0.1.285.iso"
SSH_PUB_KEY_FILE="$HOME/.lima/_config/user.pub"
VIRTIOFSD_PID_FILE="$SCRIPT_DIR/virtiofsd.sock.pid"

cleanup() {
    if [[ -f "$VIRTIOFSD_PID_FILE" ]]; then
        kill "$(cat "$VIRTIOFSD_PID_FILE")" 2>/dev/null || true
        rm -f "$VIRTIOFSD_PID_FILE"
    fi
    rm -f "$SCRIPT_DIR/virtiofsd.sock"
}
trap cleanup EXIT

check_prereqs() {
    local missing=()
    for cmd in qemu-system-x86_64 qemu-img virtiofsd; do
        command -v "$cmd" &>/dev/null || missing+=("$cmd")
    done
    for cmd in genisoimage mkisofs xorriso; do
        command -v "$cmd" &>/dev/null && { ISO_TOOL="$cmd"; break; }
    done
    [[ -z "${ISO_TOOL:-}" ]] && missing+=("genisoimage (or mkisofs / xorriso)")

    if [[ "${#missing[@]}" -gt 0 ]]; then
        fatal "Missing tools: ${missing[*]}\n  Ubuntu: sudo apt install ${missing[*]//genisoimage*/genisoimage}"
    fi

    [[ -f "$WIN_ISO" ]]    || fatal "Windows ISO not found at: $WIN_ISO"
    [[ -f "$VIRTIO_ISO" ]] || fatal "VirtIO-Win ISO not found at: $VIRTIO_ISO"
    [[ -f "$SSH_PUB_KEY_FILE" ]] || fatal "SSH public key not found at: $SSH_PUB_KEY_FILE"
}

build_cidata_iso() {
    info "Building win-cidata.iso"
    rm -rf "$TMP_DIR"
    mkdir -p "$TMP_DIR"

    local ssh_key
    ssh_key="$(cat "$SSH_PUB_KEY_FILE")"

    sed "s|REPLACE_WITH_YOUR_SSH_PUBLIC_KEY|${ssh_key}|g" \
        "$SCRIPT_DIR/autounattend.xml" > "$TMP_DIR/autounattend.xml"

    cp "$SCRIPT_DIR/first_logon.ps1" "$TMP_DIR/first_logon.ps1"

    "$ISO_TOOL" -output "$CIDATA_ISO" -joliet -rock -volid cidata "$TMP_DIR" \
        2>&1 | grep -v '^$' || true

    [[ -s "$CIDATA_ISO" ]] || fatal "ISO tool produced an empty file"
    info "win-cidata.iso ready ($(du -sh "$CIDATA_ISO" | cut -f1))"
}

create_disk() {
    info "Creating 40 GB qcow2 disk"
    qemu-img create -f qcow2 "$QCOW2" 40G -q
}

start_virtiofsd() {
    info "Starting virtiofsd"
    virtiofsd \
        --shared-dir="$SCRIPT_DIR" \
        --socket-path="$SCRIPT_DIR/virtiofsd.sock" \
        --sandbox none &
    echo $! > "$VIRTIOFSD_PID_FILE"
    sleep 1
    [[ -S "$SCRIPT_DIR/virtiofsd.sock" ]] || fatal "virtiofsd socket not created"
}

run_qemu() {
    info "Launching QEMU — Windows installation will begin automatically"
    warn "First boot (OS install) takes 15-30 min depending on your machine"

    qemu-system-x86_64 \
        -name guest=win2k25,debug-threads=on \
        -machine pc-q35-noble,usb=off,vmport=off,dump-guest-core=off,memory-backend=pc.ram,hpet=off,acpi=on \
        -accel kvm \
        -cpu host,migratable=on,hv-time=on,hv-relaxed=on,hv-vapic=on,hv-spinlocks=0x1fff \
        -m size=4096 \
        -object memory-backend-memfd,id=pc.ram,share=true,size=4096M \
        -overcommit mem-lock=off \
        -smp 4,sockets=2,cores=2,threads=1 \
        -drive file="$QCOW2",if=virtio,id=disk0,discard=on \
        -blockdev driver=file,filename="$WIN_ISO",node-name=cdrom0-storage,read-only=true \
        -blockdev driver=raw,file=cdrom0-storage,node-name=cdrom0,read-only=true \
        -device ide-cd,bus=ide.1,drive=cdrom0 \
        -blockdev driver=file,filename="$VIRTIO_ISO",node-name=cdrom1-storage,read-only=true \
        -blockdev driver=raw,file=cdrom1-storage,node-name=cdrom1,read-only=true \
        -device ide-cd,bus=ide.2,drive=cdrom1 \
        -blockdev driver=file,filename="$CIDATA_ISO",node-name=cdrom2-storage,read-only=true \
        -blockdev driver=raw,file=cdrom2-storage,node-name=cdrom2,read-only=true \
        -device ide-cd,bus=ide.3,drive=cdrom2 \
        -netdev user,id=net0,net=192.168.10.0/24,dhcpstart=192.168.10.15,hostfwd=tcp:127.0.0.1:60022-:22 \
        -device virtio-net-pci,netdev=net0 \
        -chardev socket,id=chr-vu-fs0,path="$SCRIPT_DIR/virtiofsd.sock" \
        -device vhost-user-fs-pci,chardev=chr-vu-fs0,tag=lima \
        -device virtio-rng-pci
}

cd "$SCRIPT_DIR"
check_prereqs
rm -f "$QCOW2" "$CIDATA_ISO"
build_cidata_iso
create_disk
start_virtiofsd
run_qemu
