// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	continuityfs "github.com/containerd/continuity/fs"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func CloneOrRename(ctx context.Context, oldInst *limatype.Instance, newInstName string, rename bool) (*limatype.Instance, error) {
	verb := "clone"
	if rename {
		verb = "rename"
	}
	if newInstName == "" {
		return nil, errors.New("got empty instName")
	}
	if oldInst.Name == newInstName {
		return nil, fmt.Errorf("new instance name %q must be different from %q", newInstName, oldInst.Name)
	}
	if oldInst.Status == limatype.StatusRunning {
		return nil, errors.New("cannot " + verb + " a running instance")
	}

	newInstDir, err := dirnames.InstanceDir(newInstName)
	if err != nil {
		return nil, err
	}

	if _, err = os.Stat(newInstDir); !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("instance %q already exists", newInstName)
	}

	// the full path of the socket name must be less than UNIX_PATH_MAX chars.
	maxSockName := filepath.Join(newInstDir, filenames.LongestSock)
	if len(maxSockName) >= osutil.UnixPathMax {
		return nil, fmt.Errorf("instance name %q too long: %q must be less than UNIX_PATH_MAX=%d characters, but is %d",
			newInstName, maxSockName, osutil.UnixPathMax, len(maxSockName))
	}

	if err = os.Mkdir(newInstDir, 0o700); err != nil {
		return nil, err
	}

	walkDirFn := func(path string, d fs.DirEntry, err error) error {
		base := filepath.Base(path)
		if slices.Contains(filenames.SkipOnClone, base) {
			return nil
		}
		for _, ext := range filenames.TmpFileSuffixes {
			if strings.HasSuffix(path, ext) {
				return nil
			}
		}
		if err != nil {
			return err
		}
		pathRel, err := filepath.Rel(oldInst.Dir, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(newInstDir, pathRel)
		if d.IsDir() {
			return os.MkdirAll(dst, d.Type().Perm())
		}
		// NullifyOnClone contains VzIdentifier.
		// VzIdentifier file must not be just removed here, as pkg/limayaml depends on
		// the existence of VzIdentifier for resolving the VM type.
		if slices.Contains(filenames.NullifyOnClone, base) {
			return os.WriteFile(dst, nil, 0o666)
		}
		if rename {
			return os.Rename(path, dst)
		}
		// CopyFile attempts copy-on-write when supported by the filesystem
		return continuityfs.CopyFile(dst, path)
	}

	if err = filepath.WalkDir(oldInst.Dir, walkDirFn); err != nil {
		return nil, err
	}
	if rename {
		if err = os.RemoveAll(oldInst.Dir); err != nil {
			return nil, err
		}
	}
	return store.Inspect(ctx, newInstName)
}
