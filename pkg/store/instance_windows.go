package store

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/lima-vm/lima/pkg/executil"
	"github.com/lima-vm/lima/pkg/limayaml"
)

func inspectStatus(instDir string, inst *Instance, y *limayaml.LimaYAML) {
	if inst.VMType == limayaml.WSL2 {
		status, err := GetWslStatus(inst.Name)
		if err != nil {
			inst.Status = StatusBroken
			inst.Errors = append(inst.Errors, err)
		} else {
			inst.Status = status
		}

		inst.SSHLocalPort = 22

		if inst.Status == StatusRunning {
			sshAddr, err := getWslSSHAddress(inst.Name)
			if err == nil {
				inst.SSHAddress = sshAddr
			} else {
				inst.Errors = append(inst.Errors, err)
			}
		}
	} else {
		inspectStatusWithPIDFiles(instDir, inst, y)
	}
}

// GetWslStatus runs `wsl --list --verbose` and parses its output
//
// Expected output (whitespace preserved):
// PS > wsl --list --verbose
//
//	NAME      STATE           VERSION
//
// * Ubuntu    Stopped         2
func GetWslStatus(instName string) (string, error) {
	distroName := "lima-" + instName
	out, err := executil.RunUTF16leCommand([]string{
		"wsl.exe",
		"--list",
		"--verbose",
	})
	if err != nil {
		return "", fmt.Errorf("failed to run `wsl --list --verbose`, err: %w (out=%q)", err, string(out))
	}

	if len(out) == 0 {
		return StatusBroken, fmt.Errorf("failed to read instance state for instance %s, try running `wsl --list --verbose` to debug, err: %w", instName, err)
	}

	var instState string
	// wsl --list --verbose may have differernt headers depending on localization, just split by line
	for _, rows := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		cols := regexp.MustCompile(`\s+`).Split(strings.TrimSpace(rows), -1)
		nameIdx := 0
		// '*' indicates default instance
		if cols[0] == "*" {
			nameIdx = 1
		}
		if cols[nameIdx] == distroName {
			instState = cols[nameIdx+1]
			break
		}
	}

	if instState == "" {
		return StatusUninitialized, nil
	}

	return instState, nil
}

func GetSSHAddress(instName string) (string, error) {
	return getWslSSHAddress(instName)
}

// GetWslSSHAddress runs a hostname command to get the IP from inside of a wsl2 VM.
//
// Expected output (whitespace preserved, [] for optional):
// PS > wsl -d <distroName> bash -c hostname -I | cut -d' ' -f1
// 168.1.1.1 [10.0.0.1]
func getWslSSHAddress(instName string) (string, error) {
	distroName := "lima-" + instName
	cmd := exec.Command("wsl.exe", "-d", distroName, "bash", "-c", `hostname -I | cut -d ' ' -f1`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname for instance %s, err: %w (out=%q)", instName, err, string(out))
	}

	return strings.TrimSpace(string(out)), nil
}
