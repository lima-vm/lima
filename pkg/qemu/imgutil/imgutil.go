package imgutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type InfoChild struct {
	Name string `json:"name,omitempty"` // since QEMU 8.0
	Info Info   `json:"info,omitempty"` // since QEMU 8.0
}

type InfoFormatSpecific struct {
	Type string          `json:"type,omitempty"` // since QEMU 1.7
	Data json.RawMessage `json:"data,omitempty"` // since QEMU 1.7
}

func (sp *InfoFormatSpecific) Qcow2() *InfoFormatSpecificDataQcow2 {
	if sp.Type != "qcow2" {
		return nil
	}
	var x InfoFormatSpecificDataQcow2
	if err := json.Unmarshal(sp.Data, &x); err != nil {
		panic(err)
	}
	return &x
}

func (sp *InfoFormatSpecific) Vmdk() *InfoFormatSpecificDataVmdk {
	if sp.Type != "vmdk" {
		return nil
	}
	var x InfoFormatSpecificDataVmdk
	if err := json.Unmarshal(sp.Data, &x); err != nil {
		panic(err)
	}
	return &x
}

type InfoFormatSpecificDataQcow2 struct {
	Compat          string `json:"compat,omitempty"`           // since QEMU 1.7
	LazyRefcounts   bool   `json:"lazy-refcounts,omitempty"`   // since QEMU 1.7
	Corrupt         bool   `json:"corrupt,omitempty"`          // since QEMU 2.2
	RefcountBits    int    `json:"refcount-bits,omitempty"`    // since QEMU 2.3
	CompressionType string `json:"compression-type,omitempty"` // since QEMU 5.1
	ExtendedL2      bool   `json:"extended-l2,omitempty"`      // since QEMU 5.2
}

type InfoFormatSpecificDataVmdk struct {
	CreateType string                             `json:"create-type,omitempty"` // since QEMU 1.7
	CID        int                                `json:"cid,omitempty"`         // since QEMU 1.7
	ParentCID  int                                `json:"parent-cid,omitempty"`  // since QEMU 1.7
	Extents    []InfoFormatSpecificDataVmdkExtent `json:"extents,omitempty"`     // since QEMU 1.7
}

type InfoFormatSpecificDataVmdkExtent struct {
	Filename    string `json:"filename,omitempty"`     // since QEMU 1.7
	Format      string `json:"format,omitempty"`       // since QEMU 1.7
	VSize       int64  `json:"virtual-size,omitempty"` // since QEMU 1.7
	ClusterSize int    `json:"cluster-size,omitempty"` // since QEMU 1.7
}

// Info corresponds to the output of `qemu-img info --output=json FILE`
type Info struct {
	Filename              string              `json:"filename,omitempty"`                // since QEMU 1.3
	Format                string              `json:"format,omitempty"`                  // since QEMU 1.3
	VSize                 int64               `json:"virtual-size,omitempty"`            // since QEMU 1.3
	ActualSize            int64               `json:"actual-size,omitempty"`             // since QEMU 1.3
	DirtyFlag             bool                `json:"dirty-flag,omitempty"`              // since QEMU 5.2
	ClusterSize           int                 `json:"cluster-size,omitempty"`            // since QEMU 1.3
	BackingFilename       string              `json:"backing-filename,omitempty"`        // since QEMU 1.3
	FullBackingFilename   string              `json:"full-backing-filename,omitempty"`   // since QEMU 1.3
	BackingFilenameFormat string              `json:"backing-filename-format,omitempty"` // since QEMU 1.3
	FormatSpecific        *InfoFormatSpecific `json:"format-specific,omitempty"`         // since QEMU 1.7
	Children              []InfoChild         `json:"children,omitempty"`                // since QEMU 8.0
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

func ParseInfo(b []byte) (*Info, error) {
	var imgInfo Info
	if err := json.Unmarshal(b, &imgInfo); err != nil {
		return nil, err
	}
	return &imgInfo, nil
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
	return ParseInfo(stdout.Bytes())
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
