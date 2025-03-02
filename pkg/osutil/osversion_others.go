//go:build !darwin

/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package osutil

import (
	"errors"

	"github.com/coreos/go-semver/semver"
)

// ProductVersion returns the OS product version, not the kernel version.
func ProductVersion() (*semver.Version, error) {
	return nil, errors.New("not implemented")
}
