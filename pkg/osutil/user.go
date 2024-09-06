package osutil

import (
	"fmt"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/lima-vm/lima/pkg/version/versionutil"
	"github.com/sirupsen/logrus"
)

type User struct {
	User  string
	Uid   uint32
	Group string
	Gid   uint32
	Home  string
}

type Group struct {
	Name string
	Gid  uint32
}

var (
	users  map[string]User
	groups map[string]Group
)

// regexUsername matches user and group names to be valid for `useradd`.
// `useradd` allows names with a trailing '$', but it feels prudent to map those
// names to the fallback user as well, so the regex does not allow them.
var (
	regexUsername           = regexp.MustCompile("^[a-z_][a-z0-9_-]*$")
	notAllowedUsernameRegex = regexp.MustCompile(`^admin$`)
)

// regexPath detects valid Linux path.
var regexPath = regexp.MustCompile("^[/a-zA-Z0-9_-]+$")

func LookupUser(name string) (User, error) {
	if users == nil {
		users = make(map[string]User)
	}
	if _, ok := users[name]; !ok {
		u, err := user.Lookup(name)
		if err != nil {
			return User{}, err
		}
		g, err := user.LookupGroupId(u.Gid)
		if err != nil {
			return User{}, err
		}
		uid, err := parseUidGid(u.Uid)
		if err != nil {
			return User{}, err
		}
		gid, err := parseUidGid(u.Gid)
		if err != nil {
			return User{}, err
		}
		users[name] = User{User: u.Username, Uid: uid, Group: g.Name, Gid: gid, Home: u.HomeDir}
	}
	return users[name], nil
}

func LookupGroup(name string) (Group, error) {
	if groups == nil {
		groups = make(map[string]Group)
	}
	if _, ok := groups[name]; !ok {
		g, err := user.LookupGroup(name)
		if err != nil {
			return Group{}, err
		}
		gid, err := parseUidGid(g.Gid)
		if err != nil {
			return Group{}, err
		}
		groups[name] = Group{Name: g.Name, Gid: gid}
	}
	return groups[name], nil
}

const (
	fallbackUser = "lima"
	fallbackUid  = 1000
	fallbackGid  = 1000
)

var cache struct {
	sync.Once
	u        *user.User
	err      error
	warnings []string
}

func call(args []string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func IsBlockedUsername(username, limaVersion string) bool {
	if versionutil.GreaterThan(limaVersion, "0.23.2") {
		return notAllowedUsernameRegex.MatchString(username)
	}
	return false
}

func LimaUser(warn bool, limaVersion string) (*user.User, error) {
	cache.warnings = []string{}
	cache.Do(func() {
		cache.u, cache.err = user.Current()
		if cache.err == nil {
			//	check if the username is blocked
			if IsBlockedUsername(cache.u.Username, limaVersion) {
				warning := fmt.Sprintf("local user %q is not a allowed (must not match %q); using %q username instead",
					cache.u.Username, notAllowedUsernameRegex.String(), fallbackUser)
				cache.warnings = append(cache.warnings, warning)
				cache.u.Username = fallbackUser
			}

			if !regexUsername.MatchString(cache.u.Username) {
				warning := fmt.Sprintf("local user %q is not a valid Linux username (must match %q); using %q username instead",
					cache.u.Username, regexUsername.String(), fallbackUser)
				cache.warnings = append(cache.warnings, warning)
				cache.u.Username = fallbackUser
			}
			if runtime.GOOS == "windows" {
				idu, err := call([]string{"id", "-u"})
				if err != nil {
					logrus.Debug(err)
				}
				uid, err := parseUidGid(idu)
				if err != nil {
					uid = fallbackUid
				}
				if _, err := parseUidGid(cache.u.Uid); err != nil {
					warning := fmt.Sprintf("local uid %q is not a valid Linux uid (must be integer); using %d uid instead",
						cache.u.Uid, uid)
					cache.warnings = append(cache.warnings, warning)
					cache.u.Uid = formatUidGid(uid)
				}
				idg, err := call([]string{"id", "-g"})
				if err != nil {
					logrus.Debug(err)
				}
				gid, err := parseUidGid(idg)
				if err != nil {
					gid = fallbackGid
				}
				if _, err := parseUidGid(cache.u.Gid); err != nil {
					warning := fmt.Sprintf("local gid %q is not a valid Linux gid (must be integer); using %d gid instead",
						cache.u.Gid, gid)
					cache.warnings = append(cache.warnings, warning)
					cache.u.Gid = formatUidGid(gid)
				}
				home, err := call([]string{"cygpath", cache.u.HomeDir})
				if err != nil {
					logrus.Debug(err)
				}
				if home == "" {
					drive := filepath.VolumeName(cache.u.HomeDir)
					home = filepath.ToSlash(cache.u.HomeDir)
					// replace C: with /c
					prefix := strings.ToLower(fmt.Sprintf("/%c", drive[0]))
					home = strings.Replace(home, drive, prefix, 1)
				}
				if !regexPath.MatchString(cache.u.HomeDir) {
					warning := fmt.Sprintf("local home %q is not a valid Linux path (must match %q); using %q home instead",
						cache.u.HomeDir, regexPath.String(), home)
					cache.warnings = append(cache.warnings, warning)
					cache.u.HomeDir = home
				}
			}
		}
	})
	if warn && len(cache.warnings) > 0 {
		for _, warning := range cache.warnings {
			logrus.Warn(warning)
		}
	}
	return cache.u, cache.err
}

// parseUidGid converts string value to Linux uid or gid.
func parseUidGid(uidOrGid string) (uint32, error) {
	res, err := strconv.ParseUint(uidOrGid, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(res), nil
}

// formatUidGid converts uid or gid to string value.
func formatUidGid(uidOrGid uint32) string {
	return strconv.FormatUint(uint64(uidOrGid), 10)
}
