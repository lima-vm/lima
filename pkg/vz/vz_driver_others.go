//go:build !darwin || no_vz

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

package vz

import (
	"context"
	"errors"

	"github.com/lima-vm/lima/pkg/driver"
)

var ErrUnsupported = errors.New("vm driver 'vz' needs macOS 13 or later (Hint: try recompiling Lima if you are seeing this error on macOS 13)")

const Enabled = false

type LimaVzDriver struct {
	*driver.BaseDriver
}

func New(driver *driver.BaseDriver) *LimaVzDriver {
	return &LimaVzDriver{
		BaseDriver: driver,
	}
}

func (l *LimaVzDriver) Validate() error {
	return ErrUnsupported
}

func (l *LimaVzDriver) CreateDisk(_ context.Context) error {
	return ErrUnsupported
}

func (l *LimaVzDriver) Start(_ context.Context) (chan error, error) {
	return nil, ErrUnsupported
}

func (l *LimaVzDriver) Stop(_ context.Context) error {
	return ErrUnsupported
}
