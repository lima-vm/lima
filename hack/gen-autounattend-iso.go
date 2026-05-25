// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// gen-autounattend-iso generates an autounattend.iso for testing the Windows
// guest POC with QEMU.
// Usage: go run hack/gen-autounattend-iso.go [-arch amd64|arm64] [-o autounattend.iso]
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/lima-vm/lima/v2/pkg/cidata"
	"github.com/lima-vm/lima/v2/pkg/iso9660util"
)

func main() {
	outPath := flag.String("o", "autounattend.iso", "output ISO file path")
	arch := flag.String("arch", "amd64", "target architecture: amd64 or arm64")
	flag.Parse()

	if *arch != "amd64" && *arch != "arm64" {
		fmt.Fprintf(os.Stderr, "error: -arch must be amd64 or arm64, got %q\n", *arch)
		os.Exit(1)
	}

	args := &cidata.TemplateArgs{
		Name: "windows-test",
	}

	xmlBytes, err := cidata.ExecuteTemplateAutounattend(args, *arch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error rendering template: %v\n", err)
		os.Exit(1)
	}

	// Also write the raw XML for inspection
	xmlPath := *outPath + ".xml"
	if err := os.WriteFile(xmlPath, xmlBytes, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing XML: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "Wrote %s (%d bytes)\n", xmlPath, len(xmlBytes))

	layout := []iso9660util.Entry{
		{
			Path:   "autounattend.xml",
			Reader: bytes.NewReader(xmlBytes),
		},
	}

	// Only include startup.nsh for amd64 (EFI Shell fallback for OVMF on x86_64)
	if *arch == "amd64" {
		layout = append(layout, iso9660util.Entry{
			Path: "startup.nsh",
			Reader: bytes.NewReader([]byte("@echo -off\r\n" +
				"echo Searching for Windows Boot Manager...\r\n" +
				"FS0:\\EFI\\BOOT\\BOOTX64.EFI\r\n" +
				"FS1:\\EFI\\BOOT\\BOOTX64.EFI\r\n" +
				"FS2:\\EFI\\BOOT\\BOOTX64.EFI\r\n" +
				"FS3:\\EFI\\BOOT\\BOOTX64.EFI\r\n" +
				"FS4:\\EFI\\BOOT\\BOOTX64.EFI\r\n")),
		})
	}

	if err := iso9660util.Write(*outPath, "autounattend", layout, iso9660util.WithJoliet()); err != nil {
		fmt.Fprintf(os.Stderr, "error writing ISO: %v\n", err)
		os.Exit(1)
	}

	fi, _ := os.Stat(*outPath)
	fmt.Fprintf(os.Stdout, "Wrote %s (%d bytes)\n", *outPath, fi.Size())

	if *arch == "amd64" {
		printAmd64Usage(*outPath)
	} else {
		printArm64Usage(*outPath)
	}
}

func printAmd64Usage(outPath string) {
	fmt.Fprintln(os.Stdout, "\nUsage with QEMU amd64 (UEFI boot required):")
	fmt.Fprintln(os.Stdout, "  qemu-img create -f qcow2 disk.qcow2 64G")
	fmt.Fprintln(os.Stdout, "  dd if=/dev/zero of=ovmf_vars.fd bs=1M count=4")
	fmt.Fprintln(os.Stdout, "  qemu-system-x86_64 -m 4G -smp 2 -machine q35 \\")
	fmt.Fprintln(os.Stdout, "    -drive if=pflash,format=raw,readonly=on,file=/usr/share/OVMF/OVMF_CODE_4M.fd \\")
	fmt.Fprintln(os.Stdout, "    -drive if=pflash,format=raw,file=ovmf_vars.fd \\")
	fmt.Fprintln(os.Stdout, "    -cdrom /path/to/windows.iso \\")
	fmt.Fprintf(os.Stdout, "    -drive file=%s,media=cdrom,index=3 \\\n", outPath)
	fmt.Fprintln(os.Stdout, "    -drive file=disk.qcow2,index=0,media=disk,format=qcow2 \\")
	fmt.Fprintln(os.Stdout, "    -nic user,hostfwd=tcp::2222-:22")
	fmt.Fprintln(os.Stdout, "\n  On macOS (Homebrew): replace OVMF paths with")
	fmt.Fprintln(os.Stdout, "    /opt/homebrew/share/qemu/edk2-x86_64-code.fd")
	fmt.Fprintln(os.Stdout, "\n  On Windows (PowerShell, WHPX): add -accel whpx")
}

func printArm64Usage(outPath string) {
	fmt.Fprintln(os.Stdout, "\nUsage with QEMU aarch64 on Apple Silicon (HVF):")
	fmt.Fprintln(os.Stdout, "  qemu-img create -f qcow2 disk.qcow2 64G")
	fmt.Fprintln(os.Stdout, "  dd if=/dev/zero of=ovmf_vars.fd bs=1M count=64")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "  qemu-system-aarch64 \\")
	fmt.Fprintln(os.Stdout, "    -machine virt,accel=hvf \\")
	fmt.Fprintln(os.Stdout, "    -cpu host \\")
	fmt.Fprintln(os.Stdout, "    -m 4G -smp 4 \\")
	fmt.Fprintln(os.Stdout, "    -drive if=pflash,format=raw,readonly=on,file=/opt/homebrew/share/qemu/edk2-aarch64-code.fd \\")
	fmt.Fprintln(os.Stdout, "    -drive if=pflash,format=raw,file=ovmf_vars.fd \\")
	fmt.Fprintln(os.Stdout, "    -drive file=disk.qcow2,if=none,id=hd0,format=qcow2 \\")
	fmt.Fprintln(os.Stdout, "    -device nvme,serial=lima0,drive=hd0 \\")
	fmt.Fprintln(os.Stdout, "    -device qemu-xhci \\")
	fmt.Fprintln(os.Stdout, "    -device usb-kbd \\")
	fmt.Fprintln(os.Stdout, "    -device usb-tablet \\")
	fmt.Fprintln(os.Stdout, "    -drive file=/path/to/Win11_ARM64.iso,if=none,id=cdrom0,media=cdrom,readonly=on \\")
	fmt.Fprintln(os.Stdout, "    -device usb-storage,drive=cdrom0 \\")
	fmt.Fprintf(os.Stdout, "    -drive file=%s,if=none,id=unattend,media=cdrom,readonly=on \\\n", outPath)
	fmt.Fprintln(os.Stdout, "    -device usb-storage,drive=unattend \\")
	fmt.Fprintln(os.Stdout, "    -device ramfb \\")
	fmt.Fprintln(os.Stdout, "    -nic user,hostfwd=tcp::2222-:22")
}
