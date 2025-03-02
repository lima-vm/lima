// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

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

	"github.com/lima-vm/lima/pkg/ioutilx"
	. "github.com/lima-vm/lima/pkg/must"
	"github.com/lima-vm/lima/pkg/version/versionutil"
	"github.com/sirupsen/logrus"
)

type User struct {
	User  string
	Uid   uint32
	Group string
	Gid   uint32
	Name  string // or Comment
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
		uid, err := parseUidGid(u.Uid)
		if err != nil {
			return User{}, err
		}
		gid, err := parseUidGid(u.Gid)
		if err != nil {
			return User{}, err
		}
		users[name] = User{User: u.Username, Uid: uid, Group: g.Name, Gid: gid, Name: u.Name, Home: u.HomeDir}
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

var currentUser = Must(user.Current())

var (
	once     = new(sync.Once)
	limaUser *user.User
	warnings []string
)

func LimaUser(limaVersion string, warn bool) *user.User {
	once.Do(func() {
		limaUser = currentUser
		if !regexUsername.MatchString(limaUser.Username) {
			warning := fmt.Sprintf("local username %q is not a valid Linux username (must match %q); using %q instead",
				limaUser.Username, regexUsername.String(), fallbackUser)
			warnings = append(warnings, warning)
			limaUser.Username = fallbackUser
		}
		if runtime.GOOS != "windows" {
			limaUser.HomeDir = "/home/{{.User}}.linux"
		} else {
			idu, err := call([]string{"id", "-u"})
			if err != nil {
				logrus.Debug(err)
			}
			uid, err := parseUidGid(idu)
			if err != nil {
				uid = fallbackUid
			}
			if _, err := parseUidGid(limaUser.Uid); err != nil {
				warning := fmt.Sprintf("local uid %q is not a valid Linux uid (must be integer); using %d uid instead",
					limaUser.Uid, uid)
				warnings = append(warnings, warning)
				limaUser.Uid = formatUidGid(uid)
			}
			idg, err := call([]string{"id", "-g"})
			if err != nil {
				logrus.Debug(err)
			}
			gid, err := parseUidGid(idg)
			if err != nil {
				gid = fallbackGid
			}
			if _, err := parseUidGid(limaUser.Gid); err != nil {
				warning := fmt.Sprintf("local gid %q is not a valid Linux gid (must be integer); using %d gid instead",
					limaUser.Gid, gid)
				warnings = append(warnings, warning)
				limaUser.Gid = formatUidGid(gid)
			}
			home, err := ioutilx.WindowsSubsystemPath(limaUser.HomeDir)
			if err != nil {
				logrus.Debug(err)
			} else {
				home += ".linux"
			}
			if home == "" {
				drive := filepath.VolumeName(limaUser.HomeDir)
				home = filepath.ToSlash(limaUser.HomeDir)
				// replace C: with /c
				prefix := strings.ToLower(fmt.Sprintf("/%c", drive[0]))
				home = strings.Replace(home, drive, prefix, 1)
				home += ".linux"
			}
			if !regexPath.MatchString(limaUser.HomeDir) {
				// Trim prefix of well known default mounts
				if strings.HasPrefix(home, "/mnt/") {
					home = strings.TrimPrefix(home, "/mnt")
				}
				warning := fmt.Sprintf("local home %q is not a valid Linux path (must match %q); using %q home instead",
					limaUser.HomeDir, regexPath.String(), home)
				warnings = append(warnings, warning)
				limaUser.HomeDir = home
			}
		}
	})
	if warn {
		for _, warning := range warnings {
			logrus.Warn(warning)
		}
	}
	// Make sure we return a pointer to a COPY of limaUser
	u := *limaUser
	if versionutil.GreaterEqual(limaVersion, "1.0.0") {
		if u.Username == "admin" {
			if warn {
				logrus.Warnf("local username %q is reserved; using %q instead", u.Username, fallbackUser)
			}
			u.Username = fallbackUser
		}
	}
	return &u
}

func call(args []string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
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
