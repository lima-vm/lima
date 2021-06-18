// Package filenames defines the names of the files that appear under an instance dir.
//
// See docs/internal.md .
package filenames

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
)

// LongestSock is the longest socket name.
// On macOS, the full path of the socket can be at most 104 characters.
// See unix(4).
//
// On Linux, the max length is 108.
const LongestSock = SerialSock
