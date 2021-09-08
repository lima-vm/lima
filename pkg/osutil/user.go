package osutil

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"os/user"
	"sync"

	"github.com/containerd/containerd/identifiers"
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
			if err := identifiers.Validate(cache.u.Username); err != nil {
				cache.warning = fmt.Sprintf("local user %q is not a valid Linux username: %v; using %q username instead",
					cache.u.Username, err, fallbackUser)
				cache.u.Username = fallbackUser
			}
		}
	})
	if warn && cache.warning != "" {
		logrus.Warn(cache.warning)
	}
	return cache.u, cache.err
}
