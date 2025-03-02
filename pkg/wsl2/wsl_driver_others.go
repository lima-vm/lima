//go:build !windows || no_wsl

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

package wsl2

import (
	"context"
	"errors"

	"github.com/lima-vm/lima/pkg/driver"
)

var ErrUnsupported = errors.New("vm driver 'wsl2' requires Windows 10 build 19041 or later (Hint: try recompiling Lima if you are seeing this error on Windows 10+)")

const Enabled = false

type LimaWslDriver struct {
	*driver.BaseDriver
}

func New(driver *driver.BaseDriver) *LimaWslDriver {
	return &LimaWslDriver{
		BaseDriver: driver,
	}
}

func (l *LimaWslDriver) Validate() error {
	return ErrUnsupported
}

func (l *LimaWslDriver) CreateDisk(_ context.Context) error {
	return ErrUnsupported
}

func (l *LimaWslDriver) Start(_ context.Context) (chan error, error) {
	return nil, ErrUnsupported
}

func (l *LimaWslDriver) Stop(_ context.Context) error {
	return ErrUnsupported
}
