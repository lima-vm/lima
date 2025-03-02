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

package infoutil

import (
	"github.com/lima-vm/lima/pkg/driverutil"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/templatestore"
	"github.com/lima-vm/lima/pkg/version"
)

type Info struct {
	Version         string                   `json:"version"`
	Templates       []templatestore.Template `json:"templates"`
	DefaultTemplate *limayaml.LimaYAML       `json:"defaultTemplate"`
	LimaHome        string                   `json:"limaHome"`
	VMTypes         []string                 `json:"vmTypes"` // since Lima v0.14.2
}

func GetInfo() (*Info, error) {
	b, err := templatestore.Read(templatestore.Default)
	if err != nil {
		return nil, err
	}
	y, err := limayaml.Load(b, "")
	if err != nil {
		return nil, err
	}
	info := &Info{
		Version:         version.Version,
		DefaultTemplate: y,
		VMTypes:         driverutil.Drivers(),
	}
	info.Templates, err = templatestore.Templates()
	if err != nil {
		return nil, err
	}
	info.LimaHome, err = dirnames.LimaDir()
	if err != nil {
		return nil, err
	}
	return info, nil
}
