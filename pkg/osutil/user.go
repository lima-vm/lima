package osutil

import (
	"fmt"
	"os/user"
	"regexp"
	"strconv"
	"sync"

	"github.com/sirupsen/logrus"
)

type User struct {
	User  string
	Uid   uint32
	Group string
	Gid   uint32
}

type Group struct {
	Name string
	Gid  uint32
}

var users map[string]User
var groups map[string]Group

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
		users[name] = User{User: u.Username, Uid: uint32(uid), Group: g.Name, Gid: uint32(gid)}
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
)

var cache struct {
	sync.Once
	u       *user.User
	err     error
	warning string
}

func LimaUser(warn bool) (*user.User, error) {
	cache.Do(func() {
		cache.u, cache.err = user.Current()
		if cache.err == nil {
			// `useradd` only allows user and group names matching the following pattern:
			// (it allows a trailing '$', but it feels prudent to map those to the fallback user as well)
			validName := "^[a-z_][a-z0-9_-]*$"
			if !regexp.MustCompile(validName).Match([]byte(cache.u.Username)) {
				cache.warning = fmt.Sprintf("local user %q is not a valid Linux username (must match %q); using %q username instead",
					cache.u.Username, validName, fallbackUser)
				cache.u.Username = fallbackUser
			}
		}
	})
	if warn && cache.warning != "" {
		logrus.Warn(cache.warning)
	}
	return cache.u, cache.err
}
