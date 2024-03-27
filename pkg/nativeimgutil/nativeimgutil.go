// Package nativeimgutil provides image utilities that do not depend on `qemu-img` binary.
package nativeimgutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/containerd/continuity/fs"
	"github.com/docker/go-units"
	"github.com/lima-vm/go-qcow2reader"
	"github.com/lima-vm/go-qcow2reader/image/qcow2"
	"github.com/lima-vm/go-qcow2reader/image/raw"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/progressbar"
	"github.com/sirupsen/logrus"
)

// ConvertToRaw converts a source disk into a raw disk.
// source and dest may be same.
// ConvertToRaw is a NOP if source == dest, and no resizing is needed.
func ConvertToRaw(source, dest string, size *int64, allowSourceWithBackingFile bool) error {
	srcF, err := os.Open(source)
	if err != nil {
		return err
	}
	defer srcF.Close()
	srcImg, err := qcow2reader.Open(srcF)
	if err != nil {
		return fmt.Errorf("failed to detect the format of %q: %w", source, err)
	}
	if size != nil && *size < srcImg.Size() {
		return fmt.Errorf("specified size %d is smaller than the original image size (%d) of %q", *size, srcImg.Size(), source)
	}
	logrus.Infof("Converting %q (%s) to a raw disk %q", source, srcImg.Type(), dest)
	switch t := srcImg.Type(); t {
	case raw.Type:
		if err = srcF.Close(); err != nil {
			return err
		}
		return convertRawToRaw(source, dest, size)
	case qcow2.Type:
		if !allowSourceWithBackingFile {
			q, ok := srcImg.(*qcow2.Qcow2)
			if !ok {
				return fmt.Errorf("unexpected qcow2 image %T", srcImg)
			}
			if q.BackingFile != "" {
				return fmt.Errorf("qcow2 image %q has an unexpected backing file: %q", source, q.BackingFile)
			}
		}
	default:
		logrus.Warnf("image %q has an unexpected format: %q", source, t)
	}
	if err = srcImg.Readable(); err != nil {
		return fmt.Errorf("image %q is not readable: %w", source, err)
	}

	// Create a tmp file because source and dest can be same.
	destTmpF, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".lima-*.tmp")
	if err != nil {
		return err
	}
	destTmp := destTmpF.Name()
	defer os.RemoveAll(destTmp)
	defer destTmpF.Close()

	// Copy
	srcImgR := io.NewSectionReader(srcImg, 0, srcImg.Size())
	bar, err := progressbar.New(srcImg.Size())
	if err != nil {
		return err
	}
	const bufSize = 1024 * 1024
	bar.Start()
	copied, err := copySparse(destTmpF, bar.NewProxyReader(srcImgR), bufSize)
	bar.Finish()
	if err != nil {
		return fmt.Errorf("failed to call copySparse(), bufSize=%d, copied=%d: %w", bufSize, copied, err)
	}

	// Resize
	if size != nil {
		logrus.Infof("Expanding to %s", units.BytesSize(float64(*size)))
		if err = MakeSparse(destTmpF, *size); err != nil {
			return err
		}
	}
	if err = destTmpF.Close(); err != nil {
		return err
	}

	// Rename destTmp into dest
	if err = os.RemoveAll(dest); err != nil {
		return err
	}
	return os.Rename(destTmp, dest)
}

func convertRawToRaw(source, dest string, size *int64) error {
	if source != dest {
		// continuity attempts clonefile
		if err := fs.CopyFile(dest, source); err != nil {
			return fmt.Errorf("failed to copy %q into %q: %w", source, dest, err)
		}
	}
	if size != nil {
		logrus.Infof("Expanding to %s", units.BytesSize(float64(*size)))
		destF, err := os.OpenFile(dest, os.O_RDWR, 0o644)
		if err != nil {
			return err
		}
		if err = MakeSparse(destF, *size); err != nil {
			_ = destF.Close()
			return err
		}
		return destF.Close()
	}
	return nil
}

func copySparse(w *os.File, r io.Reader, bufSize int64) (int64, error) {
	var (
		n              int64
		eof, hasWrites bool
	)

	zeroBuf := make([]byte, bufSize)
	buf := make([]byte, bufSize)
	for !eof {
		rN, rErr := r.Read(buf)
		if rErr != nil {
			eof = errors.Is(rErr, io.EOF)
			if !eof {
				return n, fmt.Errorf("failed to read: %w", rErr)
			}
		}
		// TODO: qcow2reader should have a method to notify whether buf is zero
		if bytes.Equal(buf, zeroBuf) {
			if _, sErr := w.Seek(int64(rN), io.SeekCurrent); sErr != nil {
				return n, fmt.Errorf("failed seek: %w", sErr)
			}
			// no need to ftruncate here
			n += int64(rN)
		} else {
			hasWrites = true
			wN, wErr := w.Write(buf)
			if wN > 0 {
				n += int64(wN)
			}
			if wErr != nil {
				return n, fmt.Errorf("failed to read: %w", wErr)
			}
			if wN != rN {
				return n, fmt.Errorf("read %d, but wrote %d bytes", rN, wN)
			}
		}
	}

	// Ftruncate must be run if the file contains only zeros
	if !hasWrites {
		return n, MakeSparse(w, n)
	}
	return n, nil
}

func MakeSparse(f *os.File, n int64) error {
	if _, err := f.Seek(n, io.SeekStart); err != nil {
		return err
	}
	return osutil.Ftruncate(int(f.Fd()), n)
}
