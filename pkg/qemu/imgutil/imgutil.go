package imgutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Info corresponds to the output of `qemu-img info --output=json FILE`
type Info struct {
	Format string `json:"format,omitempty"` // since QEMU 1.3
	VSize  int64  `json:"virtual-size,omitempty"`
}

func ConvertToRaw(source string, dest string) error {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("qemu-img", "convert", "-O", "raw", source, dest)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %v: stdout=%q, stderr=%q: %w",
			cmd.Args, stdout.String(), stderr.String(), err)
	}
	return nil
}

func GetInfo(f string) (*Info, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("qemu-img", "info", "--output=json", "--force-share", f)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run %v: stdout=%q, stderr=%q: %w",
			cmd.Args, stdout.String(), stderr.String(), err)
	}
	var imgInfo Info
	if err := json.Unmarshal(stdout.Bytes(), &imgInfo); err != nil {
		return nil, err
	}
	return &imgInfo, nil
}

func DetectFormat(f string) (string, error) {
	switch ext := strings.ToLower(filepath.Ext(f)); ext {
	case ".qcow2":
		return "qcow2", nil
	case ".raw":
		return "raw", nil
	}
	imgInfo, err := GetInfo(f)
	if err != nil {
		return "", err
	}
	if imgInfo.Format == "" {
		return "", fmt.Errorf("failed to detect format of %q", f)
	}
	return imgInfo.Format, nil
}
