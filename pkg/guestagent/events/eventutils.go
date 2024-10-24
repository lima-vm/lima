//go:build linux
// +build linux

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

func sendHostAgentEvent(remove bool, ipPorts []*api.IPPort, ch chan *api.Event) {
	ev := &api.Event{
		Time: timestamppb.Now(),
	}
	if remove {
		ev.RemovedLocalPorts = ipPorts
	} else {
		ev.AddedLocalPorts = ipPorts
	}
	ch <- ev
	logrus.Debugf("sent the following event to hostAgent: %+v", ev)
}

type NoClientError struct{}

func (e *NoClientError) Error() string {
	return "no available client connected to the event monitor"
}

func (e *NoClientError) Is(target error) bool {
	_, ok := target.(*NoClientError)
	return ok
}
