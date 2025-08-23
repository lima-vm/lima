// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package filenames defines the names of the files that appear under an instance dir
// or inside the config directory.
//
// See https://lima-vm.io/docs/dev/internals/
package filenames

// Instance names starting with an underscore are reserved for lima internal usage

const (
	CacheDir     = "_cache" // not yet implemented
	ConfigDir    = "_config"
	DisksDir     = "_disks"     // disks are stored here
	NetworksDir  = "_networks"  // network log files are stored here
	TemplatesDir = "_templates" // user templates are stored here
)

// Filenames used inside the ConfigDir

const (
	UserPrivateKey = "user"
	UserPublicKey  = UserPrivateKey + ".pub"
	NetworksConfig = "networks.yaml"
	Default        = "default.yaml"
	Override       = "override.yaml"
	Base           = "base.yaml"
)

// Filenames that may appear under an instance directory

const (
	LimaYAML                = "lima.yaml"
	LimaVersion             = "lima-version" // Lima version used to create instance
	CIDataISO               = "cidata.iso"
	CIDataISODir            = "cidata"
	CloudConfig             = "cloud-config.yaml"
	BaseDisk                = "basedisk"
	DiffDisk                = "diffdisk"
	Kernel                  = "kernel"
	KernelCmdline           = "kernel.cmdline"
	Initrd                  = "initrd"
	QMPSock                 = "qmp.sock"
	SerialLog               = "serial.log" // default serial (ttyS0, but ttyAMA0 on qemu-system-{arm,aarch64})
	SerialSock              = "serial.sock"
	SerialPCILog            = "serialp.log" // pci serial (ttyS0 on qemu-system-{arm,aarch64})
	SerialPCISock           = "serialp.sock"
	SerialVirtioLog         = "serialv.log" // virtio serial
	SerialVirtioSock        = "serialv.sock"
	SSHSock                 = "ssh.sock"
	SSHConfig               = "ssh.config"
	VhostSock               = "virtiofsd-%d.sock"
	VNCDisplayFile          = "vncdisplay"
	VNCPasswordFile         = "vncpassword"
	GuestAgentSock          = "ga.sock"
	VirtioPort              = "io.lima-vm.guest_agent.0"
	HostAgentPID            = "ha.pid"
	HostAgentSock           = "ha.sock"
	HostAgentStdoutLog      = "ha.stdout.log"
	HostAgentStderrLog      = "ha.stderr.log"
	ExternalDriverStderrLog = "driver.stderr.log"
	VzIdentifier            = "vz-identifier"
	VzEfi                   = "vz-efi"           // efi variable store
	QemuEfiCodeFD           = "qemu-efi-code.fd" // efi code; not always created
	QemuEfiFullFD           = "qemu-efi-full.fd" // concatenated efi vars and code; not always created
	AnsibleInventoryYAML    = "ansible-inventory.yaml"

	// SocketDir is the default location for forwarded sockets with a relative paths in HostSocket.
	SocketDir = "sock"

	Protected = "protected" // empty file; used by `limactl protect`
)

// Filenames used under a disk directory

const (
	DataDisk = "datadisk"
	InUseBy  = "in_use_by"
)

// LongestSock is the longest socket name.
// On macOS, the full path of the socket (excluding the NUL terminator) must be less than 104 characters.
// See unix(4).
//
// On Linux, the full path must be less than 108 characters.
//
// ssh appends 16 bytes of random characters when it first creates the socket:
// https://github.com/openssh/openssh-portable/blob/V_8_7_P1/mux.c#L1271-L1285
const LongestSock = SSHSock + ".1234567890123456"

func PIDFile(name string) string {
	return name + ".pid"
}

// SkipOnClone files should be skipped on cloning an instance.
var SkipOnClone = []string{
	Protected,
}

// NullifyOnClone files should be nullified on cloning an instance.
// FIXME: this list should be provided by the VM driver.
var NullifyOnClone = []string{
	VzIdentifier,
}

// TmpFileSuffixes is the list of the tmp file suffixes.
var TmpFileSuffixes = []string{".pid", ".sock", ".tmp"}
