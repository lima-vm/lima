// Package filenames defines the names of the files that appear under an instance dir
// or inside the config directory.
//
// See docs/internal.md .
package filenames

// Instance names starting with an underscore are reserved for lima internal usage

const (
	ConfigDir   = "_config"
	CacheDir    = "_cache"    // not yet implemented
	NetworksDir = "_networks" // network log files are stored here
)

// Filenames used inside the ConfigDir

const (
	UserPrivateKey = "user"
	UserPublicKey  = UserPrivateKey + ".pub"
	NetworksConfig = "networks.yaml"
	Default        = "default.yaml"
	Override       = "override.yaml"
)

// Filenames that may appear under an instance directory

const (
	LimaYAML           = "lima.yaml"
	CIDataISO          = "cidata.iso"
	BaseDisk           = "basedisk"
	DiffDisk           = "diffdisk"
	DataDisk           = "datadisk"
	QemuPID            = "qemu.pid"
	QMPSock            = "qmp.sock"
	SerialLog          = "serial.log"
	SerialSock         = "serial.sock"
	SSHSock            = "ssh.sock"
	GuestAgentSock     = "ga.sock"
	HostAgentPID       = "ha.pid"
	HostAgentSock      = "ha.sock"
	HostAgentStdoutLog = "ha.stdout.log"
	HostAgentStderrLog = "ha.stderr.log"

	// SocketDir is the default location for forwarded sockets with a relative paths in HostSocket
	SocketDir = "sock"
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
