package networks

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/lima-vm/lima/pkg/osutil"
)

func (c *Config) Validate() error {
	// validate all paths.* values
	paths := reflect.ValueOf(&c.Paths).Elem()
	pathsMap := make(map[string]string, paths.NumField())
	var socketVMNetNotFound bool
	for i := 0; i < paths.NumField(); i++ {
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
		err := validatePath(path, name == "varRun")
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				switch name {
				// sudoers file does not need to exist; otherwise `limactl sudoers` couldn't bootstrap
				case "sudoers":
					continue
				case "socketVMNet":
					socketVMNetNotFound = true
					continue
				}
			}
			return fmt.Errorf("networks.yaml field `paths.%s` error: %w", name, err)
		}
	}
	if socketVMNetNotFound {
		return fmt.Errorf("networks.yaml: %q (`paths.socketVMNet`) has to be installed", pathsMap["socketVMNet"])
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
	stat, ok := osutil.SysStat(fi)
	if !ok {
		// should never happen
		return fmt.Errorf("could not retrieve stat buffer for %q", path)
	}
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("vmnet code must not be called on non-Darwin") // TODO: move to *_darwin.go
	}
	// TODO: cache looked up UIDs/GIDs
	root, err := osutil.LookupUser("root")
	if err != nil {
		return err
	}
	adminGroup, err := user.LookupGroup("admin")
	if err != nil {
		return err
	}
	adminGid, err := strconv.Atoi(adminGroup.Gid)
	if err != nil {
		return err
	}
	owner, err := user.LookupId(strconv.Itoa(int(stat.Uid)))
	if err != nil {
		return err
	}
	ownerIsAdmin := owner.Uid == "0"
	if !ownerIsAdmin {
		ownerGroupIDs, err := owner.GroupIds()
		if err != nil {
			return err
		}
		for _, g := range ownerGroupIDs {
			if g == adminGroup.Gid {
				ownerIsAdmin = true
				break
			}
		}
	}
	if !ownerIsAdmin {
		return fmt.Errorf(`%s %q owner %dis not an admin`, file, path, stat.Uid)
	}
	if allowDaemonGroupWritable {
		daemon, err := osutil.LookupUser("daemon")
		if err != nil {
			return err
		}
		if fi.Mode()&0o20 != 0 && stat.Gid != root.Gid && stat.Gid != uint32(adminGid) && stat.Gid != daemon.Gid {
			return fmt.Errorf(`%s %q is group-writable and group %d is not one of [wheel, admin, daemon]`,
				file, path, stat.Gid)
		}
		if fi.Mode().IsDir() && fi.Mode()&1 == 0 && (fi.Mode()&0o010 == 0 || stat.Gid != daemon.Gid) {
			return fmt.Errorf(`%s %q is not executable by the %q (gid: %d)" group`, file, path, daemon.User, daemon.Gid)
		}
	} else if fi.Mode()&0o20 != 0 && stat.Gid != root.Gid && stat.Gid != uint32(adminGid) {
		return fmt.Errorf(`%s %q is group-writable and group %d is not one of [wheel, admin]`,
			file, path, stat.Gid)
	}
	if fi.Mode()&0o02 != 0 {
		return fmt.Errorf("%s %q is world-writable", file, path)
	}
	if path != "/" {
		return validatePath(filepath.Dir(path), allowDaemonGroupWritable)
	}
	return nil
}
