// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networks

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"

	"github.com/lima-vm/lima/v2/pkg/identifiers"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

func (c *Config) Validate() error {
	// The group name and the per-network name/mode/interface are interpolated
	// verbatim into the sudoers file (sudoers.go) and into the socket_vmnet
	// command that reconcile.go runs via sudo after splitting it on spaces
	// (commands.go). A value with whitespace injects an extra argument, and a
	// newline in a network name adds an arbitrary directive to the generated
	// sudoers file, so require them to be valid identifiers. Interface is empty
	// for non-bridged networks, and group defaults to "admin" when unset, so
	// only validate those when a value is actually present.
	if c.Group != "" {
		if err := identifiers.Validate(c.Group); err != nil {
			return fmt.Errorf("invalid group %#q: %w", c.Group, err)
		}
	}
	for name, nw := range c.Networks {
		if err := identifiers.Validate(name); err != nil {
			return fmt.Errorf("invalid network name %#q: %w", name, err)
		}
		if nw.Mode != "" {
			if err := identifiers.Validate(nw.Mode); err != nil {
				return fmt.Errorf("invalid mode %#q for network %#q: %w", nw.Mode, name, err)
			}
		}
		if nw.Interface != "" {
			if err := identifiers.Validate(nw.Interface); err != nil {
				return fmt.Errorf("invalid interface %#q for network %#q: %w", nw.Interface, name, err)
			}
		}
	}

	// validate all paths.* values
	paths := reflect.ValueOf(&c.Paths).Elem()
	pathsMap := make(map[string]string, paths.NumField())
	var daemonNotFound string
	for i := range paths.NumField() {
		// extract YAML name from struct tag; strip options like "omitempty"
		name := paths.Type().Field(i).Tag.Get("yaml")
		if i := strings.IndexRune(name, ','); i > -1 {
			name = name[:i]
		}
		path := paths.Field(i).Interface().(string)
		pathsMap[name] = path
		// varPath will be created securely, but any existing parent directories must already be secure
		if name == "varRun" {
			path = findBaseDirectory(path)
		}
		// Only socket_vmnet runs as the "daemon" user; lima-net writes varRun as root.
		err := validatePath(path, name == "varRun" && runtime.GOOS == "darwin")
		if err != nil {
			switch {
			// sudoers file does not need to exist; otherwise `limactl sudoers` couldn't
			// bootstrap it. /etc/sudoers.d is typically not readable by the user either.
			case name == "sudoers" && (errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission)):
				continue
			case errors.Is(err, os.ErrNotExist) && slices.Contains(daemonPathFields, name):
				daemonNotFound = name
				continue
			}
			return fmt.Errorf("networks.yaml field `paths.%s` error: %w", name, err)
		}
	}
	if daemonNotFound != "" {
		return fmt.Errorf("networks.yaml: %#q (`paths.%s`) has to be installed", pathsMap[daemonNotFound], daemonNotFound)
	}
	return nil
}

// daemonPathFields are the `paths.*` fields holding a privileged helper. Only
// the one belonging to the current host OS is populated; the other is empty and
// therefore skipped by validatePath.
var daemonPathFields = []string{"socketVMNet", "limaNet"}

// findBaseDirectory removes non-existing directories from the end of the path.
func findBaseDirectory(path string) string {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		if path != "/" {
			return findBaseDirectory(filepath.Dir(path))
		}
	}
	return path
}

func validatePath(path string, allowDaemonGroupWritable bool) error {
	if path == "" {
		return nil
	}
	if path[0] != '/' {
		return fmt.Errorf("path %#q is not an absolute path", path)
	}
	if strings.ContainsRune(path, ' ') {
		return fmt.Errorf("path %#q contains whitespace", path)
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	file := "file"
	if fi.Mode().IsDir() {
		file = "dir"
	}
	// TODO: should we allow symlinks when both the link and the target are secure?
	// E.g. on macOS /var is a symlink to /private/var, /etc to /private/etc
	if (fi.Mode() & fs.ModeSymlink) != 0 {
		return fmt.Errorf("%s %#q is a symlink", file, path)
	}
	stat, ok := osutil.SysStat(fi)
	if !ok {
		// should never happen
		return fmt.Errorf("could not retrieve stat buffer for %#q", path)
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return errors.New("managed networks are only supported on macOS and Linux")
	}
	// TODO: cache looked up UIDs/GIDs
	root, err := osutil.LookupUser("root")
	if err != nil {
		return err
	}
	if stat.Uid != root.Uid {
		return fmt.Errorf(`%s %#q is not owned by %#q (uid: %d), but by uid %d`, file, path, root.User, root.Uid, stat.Uid)
	}
	if allowDaemonGroupWritable {
		daemon, err := osutil.LookupUser("daemon")
		if err != nil {
			return err
		}
		if fi.Mode()&0o20 != 0 && stat.Gid != root.Gid && stat.Gid != daemon.Gid {
			return fmt.Errorf(`%s %#q is group-writable and group is neither %#q (gid: %d) nor %#q (gid: %d), but is gid: %d`,
				file, path, root.User, root.Gid, daemon.User, daemon.Gid, stat.Gid)
		}
		if fi.Mode().IsDir() && fi.Mode()&1 == 0 && (fi.Mode()&0o010 == 0 || stat.Gid != daemon.Gid) {
			return fmt.Errorf(`%s %#q is not executable by the %#q (gid: %d)" group`, file, path, daemon.User, daemon.Gid)
		}
	} else if fi.Mode()&0o20 != 0 && stat.Gid != root.Gid {
		return fmt.Errorf(`%s %#q is group-writable and group is not %#q (gid: %d), but is gid: %d`,
			file, path, root.User, root.Gid, stat.Gid)
	}
	if fi.Mode()&0o02 != 0 {
		return fmt.Errorf("%s %#q is world-writable", file, path)
	}
	if path != "/" {
		return validatePath(filepath.Dir(path), allowDaemonGroupWritable)
	}
	return nil
}
