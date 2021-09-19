package networks

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
)

func (config *NetworksConfig) Validate() error {
	// validate all paths.* values
	paths := reflect.ValueOf(&config.Paths).Elem()
	for i := 0; i < paths.NumField(); i++ {
		// extract YAML name from struct tag; strip options like "omitempty"
		name := paths.Type().Field(i).Tag.Get("yaml")
		if i := strings.IndexRune(name, ','); i > -1 {
			name = name[:i]
		}
		path := paths.Field(i).Interface().(string)
		// varPath will be created securely, but any existing parent directories must already be secure
		if name == "varRun" {
			path = findBaseDirectory(path)
		}
		err := validatePath(path, name == "varRun")
		if err != nil {
			// sudoers file does not need to exist; otherwise `limactl sudoers` couldn't bootstrap
			if name == "sudoers" && errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("networks.yaml field `paths.%s` error: %w", name, err)
		}
	}
	// TODO(jandubois): validate network definitions
	return nil
}

// findBaseDirectory removes non-existing directories from the end of the path.
func findBaseDirectory(path string) string {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		if path != "/" {
			return findBaseDirectory(filepath.Dir(path))
		}
	}
	return path
}

const (
	RootUID   = 0
	WheelGID  = 0
	DaemonGID = 1
)

func validatePath(path string, allowDaemonGroupWritable bool) error {
	if path == "" {
		return nil
	}
	if path[0] != '/' {
		return fmt.Errorf("path %q is not an absolute path", path)
	}
	if strings.ContainsRune(path, ' ') {
		return fmt.Errorf("path %q contains whitespace", path)
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
		return fmt.Errorf("%s %q is a symlink", file, path)
	}
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		// should never happen
		return fmt.Errorf("could not retrieve stat buffer for %q", path)
	}
	if stat.Uid != RootUID {
		return fmt.Errorf(`%s %q is not owned by root (uid: %d), but by uid %d`, file, path, RootUID, stat.Uid)
	}
	if allowDaemonGroupWritable {
		if fi.Mode()&020 != 0 && stat.Gid != WheelGID && stat.Gid != DaemonGID {
			return fmt.Errorf(`%s %q is group-writable and group is neither "wheel" (gid: %d) nor "daemon" (guid: %d), but is gid: %d`,
				file, path, WheelGID, DaemonGID, stat.Gid)
		}
		if fi.Mode().IsDir() && fi.Mode()&1 == 0 && (fi.Mode()&0010 == 0 || stat.Gid != DaemonGID) {
			return fmt.Errorf(`%s %q is not executable by the "daemon" (gid: %d)" group`, file, path, DaemonGID)
		}
	} else if fi.Mode()&020 != 0 && stat.Gid != WheelGID {
		return fmt.Errorf(`%s %q is group-writable and group is not "wheel" (gid: %d), but is gid: %d`,
			file, path, WheelGID, stat.Gid)
	}
	if fi.Mode()&002 != 0 {
		return fmt.Errorf("%s %q is world-writable", file, path)
	}
	if path != "/" {
		return validatePath(filepath.Dir(path), allowDaemonGroupWritable)
	}
	return nil
}
