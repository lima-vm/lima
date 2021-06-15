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
