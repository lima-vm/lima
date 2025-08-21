// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemuimgutil

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strconv"

	"github.com/sirupsen/logrus"
)

const QemuImgFormat = "qcow2"

// QemuImageUtil is the QEMU implementation of the imgutil Interface.
type QemuImageUtil struct {
	DefaultFormat string // Default disk format, e.g., "qcow2"
}

// Info corresponds to the output of `qemu-img info --output=json FILE`.
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

type InfoChild struct {
	Name string `json:"name,omitempty"` // since QEMU 8.0
	Info Info   `json:"info,omitempty"` // since QEMU 8.0
}

type InfoFormatSpecific struct {
	Type string          `json:"type,omitempty"` // since QEMU 1.7
	Data json.RawMessage `json:"data,omitempty"` // since QEMU 1.7
}

func resizeDisk(ctx context.Context, disk, format string, size int64) error {
	args := []string{"resize", "-f", format, disk, strconv.FormatInt(size, 10)}
	cmd := exec.CommandContext(ctx, "qemu-img", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run %v: %q: %w", cmd.Args, string(out), err)
	}
	return nil
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

func convertToRaw(ctx context.Context, source, dest string) error {
	if source != dest {
		return execQemuImgConvert(ctx, source, dest)
	}

	// If source == dest, we need to use a temporary file to avoid file locking issues

	info, err := getInfo(ctx, source)
	if err != nil {
		return fmt.Errorf("failed to get info for source disk %q: %w", source, err)
	}
	if info.Format == "raw" {
		return nil
	}

	tempFile := dest + ".lima-qemu-convert.tmp"
	defer os.Remove(tempFile)

	if err := execQemuImgConvert(ctx, source, tempFile); err != nil {
		return err
	}

	return os.Rename(tempFile, dest)
}

func execQemuImgConvert(ctx context.Context, source, dest string) error {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "qemu-img", "convert", "-O", "raw", source, dest)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %v: stdout=%q, stderr=%q: %w",
			cmd.Args, stdout.String(), stderr.String(), err)
	}
	return nil
}

func parseInfo(b []byte) (*Info, error) {
	var imgInfo Info
	if err := json.Unmarshal(b, &imgInfo); err != nil {
		return nil, err
	}
	return &imgInfo, nil
}

func getInfo(ctx context.Context, f string) (*Info, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "qemu-img", "info", "--output=json", "--force-share", f)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run %v: stdout=%q, stderr=%q: %w",
			cmd.Args, stdout.String(), stderr.String(), err)
	}
	return parseInfo(stdout.Bytes())
}

// CreateDisk creates a new disk image with the specified size.
func (q *QemuImageUtil) CreateDisk(ctx context.Context, disk string, size int64) error {
	if _, err := os.Stat(disk); err == nil || !errors.Is(err, fs.ErrNotExist) {
		// disk already exists
		return err
	}

	args := []string{"create", "-f", q.DefaultFormat, disk, strconv.FormatInt(size, 10)}
	cmd := exec.CommandContext(ctx, "qemu-img", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run %v: %q: %w", cmd.Args, string(out), err)
	}
	return nil
}

// ResizeDisk resizes an existing disk image to the specified size.
func (q *QemuImageUtil) ResizeDisk(ctx context.Context, disk string, size int64) error {
	info, err := getInfo(ctx, disk)
	if err != nil {
		return fmt.Errorf("failed to get info for disk %q: %w", disk, err)
	}
	return resizeDisk(ctx, disk, info.Format, size)
}

// MakeSparse is a stub implementation as the qemu package doesn't provide this functionality.
func (q *QemuImageUtil) MakeSparse(_ context.Context, _ *os.File, _ int64) error {
	return nil
}

// GetInfo retrieves the information of a disk image.
func GetInfo(ctx context.Context, path string) (*Info, error) {
	qemuInfo, err := getInfo(ctx, path)
	if err != nil {
		return nil, err
	}

	return qemuInfo, nil
}

// ConvertToRaw converts a disk image to raw format.
func (q *QemuImageUtil) ConvertToRaw(ctx context.Context, source, dest string, size *int64, allowSourceWithBackingFile bool) error {
	if !allowSourceWithBackingFile {
		info, err := getInfo(ctx, source)
		if err != nil {
			return fmt.Errorf("failed to get info for source disk %q: %w", source, err)
		}
		if info.BackingFilename != "" || info.FullBackingFilename != "" {
			return fmt.Errorf("qcow2 image %q has an unexpected backing file: %q", source, info.BackingFilename)
		}
	}

	if err := convertToRaw(ctx, source, dest); err != nil {
		return err
	}

	if size != nil {
		destInfo, err := getInfo(ctx, dest)
		if err != nil {
			return fmt.Errorf("failed to get info for converted disk %q: %w", dest, err)
		}

		if *size > destInfo.VSize {
			return resizeDisk(ctx, dest, "raw", *size)
		}
	}

	return nil
}

// AcceptableAsBaseDisk checks if a disk image is acceptable as a base disk.
func AcceptableAsBaseDisk(info *Info) error {
	switch info.Format {
	case "qcow2", "raw":
		// NOP
	default:
		logrus.WithField("filename", info.Filename).
			Warnf("Unsupported image format %q. The image may not boot, or may have an extra privilege to access the host filesystem. Use with caution.", info.Format)
	}
	if info.BackingFilename != "" {
		return fmt.Errorf("base disk (%q) must not have a backing file (%q)", info.Filename, info.BackingFilename)
	}
	if info.FullBackingFilename != "" {
		return fmt.Errorf("base disk (%q) must not have a backing file (%q)", info.Filename, info.FullBackingFilename)
	}
	if info.FormatSpecific != nil {
		if vmdk := info.FormatSpecific.Vmdk(); vmdk != nil {
			for _, e := range vmdk.Extents {
				if e.Filename != info.Filename {
					return fmt.Errorf("base disk (%q) must not have an extent file (%q)", info.Filename, e.Filename)
				}
			}
		}
	}
	// info.Children is set since QEMU 8.0
	switch len(info.Children) {
	case 0:
	// NOP
	case 1:
		if info.Filename != info.Children[0].Info.Filename {
			return fmt.Errorf("base disk (%q) child must not have a different filename (%q)", info.Filename, info.Children[0].Info.Filename)
		}
		if len(info.Children[0].Info.Children) > 0 {
			return fmt.Errorf("base disk (%q) child must not have children of its own", info.Filename)
		}
	default:
		return fmt.Errorf("base disk (%q) must not have multiple children: %+v", info.Filename, info.Children)
	}
	return nil
}
