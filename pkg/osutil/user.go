package osutil

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"os/user"
	"regexp"
	"sync"
)

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
