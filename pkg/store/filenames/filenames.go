// Package filenames defines the names of the files that appear under an instance dir
// or inside the config directory.
//
// See docs/internal.md .
package filenames

// Instance names starting with an underscore are reserved for lima internal usage

const (
	ConfigDir = "_config"
	CacheDir  = "_cache" // not yet implemented
)

// Filenames used inside the ConfigDir

const (
	UserPrivateKey = "user"
	UserPublicKey  = UserPrivateKey + ".pub"
)

// Filenames that may appear under an instance directory

const (
	LimaYAML           = "lima.yaml"
	CIDataISO          = "cidata.iso"
	BaseDisk           = "basedisk"
	DiffDisk           = "diffdisk"
	QemuPID            = "qemu.pid"
	QMPSock            = "qmp.sock"
	SerialLog          = "serial.log"
	SerialSock         = "serial.sock"
	SSHSock            = "ssh.sock"
	GuestAgentSock     = "ga.sock"
	HostAgentPID       = "ha.pid"
	HostAgentStdoutLog = "ha.stdout.log"
	HostAgentStderrLog = "ha.stderr.log"
	Samba              = "samba.tmp"
	SambaCredentials   = "samba.tmp/credentials"
	SambaSMBConf       = "samba.tmp/smb.conf"
	SambaState         = "samba.tmp/state"
)

// LongestSock is the longest socket name.
// On macOS, the full path of the socket (excluding the NUL terminator) must be less than 104 characters.
// See unix(4).
//
// On Linux, the full path must be less than 108 characters.
const LongestSock = SerialSock
