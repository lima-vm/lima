//go:build !windows
// +build !windows

package store

import "github.com/lima-vm/lima/pkg/limayaml"

func inspectStatus(instDir string, inst *Instance, y *limayaml.LimaYAML) {
	inspectStatusWithPIDFiles(instDir, inst, y)
}

func GetSSHAddress(_ string) (string, error) {
	return "127.0.0.1", nil
}
