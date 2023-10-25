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

	"github.com/sirupsen/logrus"
)

type User struct {
	User  string
	Uid   uint32 //nolint:revive
	Group string
	Gid   uint32
	Home  string
}

type Group struct {
	Name string
	Gid  uint32
}

var users map[string]User
var groups map[string]Group

// regexUidGid detects valid Linux uid or gid.
var regexUidGid = regexp.MustCompile("^[0-9]+$") //nolint:revive

// regexUsername matches user and group names to be valid for `useradd`.
// It allows a trailing '$', but it feels prudent to map those to the fallback user as well.
var regexUsername = regexp.MustCompile("^[a-z_][a-z0-9_-]*$")

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
		uid, err := strconv.ParseUint(u.Uid, 10, 32)
		if err != nil {
			return User{}, err
		}
		gid, err := strconv.ParseUint(u.Gid, 10, 32)
		if err != nil {
			return User{}, err
		}
		users[name] = User{User: u.Username, Uid: uint32(uid), Group: g.Name, Gid: uint32(gid), Home: u.HomeDir}
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
		gid, err := strconv.ParseUint(g.Gid, 10, 32)
		if err != nil {
			return Group{}, err
		}
		groups[name] = Group{Name: g.Name, Gid: uint32(gid)}
	}
	return groups[name], nil
}

const (
	fallbackUser = "lima"
	fallbackUid  = 1000 //nolint:revive
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

func LimaUser(warn bool) (*user.User, error) {
	cache.warnings = []string{}
	cache.Do(func() {
		cache.u, cache.err = user.Current()
		if cache.err == nil {
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
				uid, err := strconv.ParseUint(idu, 10, 32)
				if err != nil {
					uid = fallbackUid
				}
				if !regexUidGid.MatchString(cache.u.Uid) {
					warning := fmt.Sprintf("local uid %q is not a valid Linux uid (must be integer); using %d uid instead",
						cache.u.Uid, uid)
					cache.warnings = append(cache.warnings, warning)
					cache.u.Uid = fmt.Sprintf("%d", uid)
				}
				idg, err := call([]string{"id", "-g"})
				if err != nil {
					logrus.Debug(err)
				}
				gid, err := strconv.ParseUint(idg, 10, 32)
				if err != nil {
					gid = fallbackGid
				}
				if !regexUidGid.MatchString(cache.u.Gid) {
					warning := fmt.Sprintf("local gid %q is not a valid Linux gid (must be integer); using %d gid instead",
						cache.u.Gid, gid)
					cache.warnings = append(cache.warnings, warning)
					cache.u.Gid = fmt.Sprintf("%d", gid)
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
