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
