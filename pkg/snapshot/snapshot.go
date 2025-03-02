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

package snapshot

import (
	"context"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/driverutil"
	"github.com/lima-vm/lima/pkg/store"
)

func Del(ctx context.Context, inst *store.Instance, tag string) error {
	limaDriver := driverutil.CreateTargetDriverInstance(&driver.BaseDriver{
		Instance: inst,
	})
	return limaDriver.DeleteSnapshot(ctx, tag)
}

func Save(ctx context.Context, inst *store.Instance, tag string) error {
	limaDriver := driverutil.CreateTargetDriverInstance(&driver.BaseDriver{
		Instance: inst,
	})
	return limaDriver.CreateSnapshot(ctx, tag)
}

func Load(ctx context.Context, inst *store.Instance, tag string) error {
	limaDriver := driverutil.CreateTargetDriverInstance(&driver.BaseDriver{
		Instance: inst,
	})
	return limaDriver.ApplySnapshot(ctx, tag)
}

func List(ctx context.Context, inst *store.Instance) (string, error) {
	limaDriver := driverutil.CreateTargetDriverInstance(&driver.BaseDriver{
		Instance: inst,
	})
	return limaDriver.ListSnapshots(ctx)
}
