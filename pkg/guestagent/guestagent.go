// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package guestagent

import (
	"context"

	"github.com/lima-vm/lima/pkg/guestagent/api"
)

type Agent interface {
	Info(ctx context.Context) (*api.Info, error)
	Events(ctx context.Context, ch chan *api.Event)
	LocalPorts(ctx context.Context) ([]*api.IPPort, error)
	HandleInotify(event *api.Inotify)
}
