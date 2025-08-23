// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limatmpl

import (
	"strings"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
)

type Template struct {
	Locator string // template locator (absolute path or URL)
	Bytes   []byte // file contents

	// The following fields are only used when the template represents a YAML config file.
	Name   string // instance name, may be inferred from locator
	Config *limatype.LimaYAML

	expr strings.Builder // yq expression to update template
}

func (tmpl *Template) ClearOnError(err error) error {
	if err != nil {
		tmpl.Bytes = nil
		tmpl.Config = nil
		tmpl.expr.Reset()
	}
	return err
}

// Unmarshal makes sure the tmpl.Config field is set. Any operation that modified
// tmpl.Bytes is expected to set tmpl.Config back to nil.
func (tmpl *Template) Unmarshal() error {
	if tmpl.Config == nil {
		tmpl.Config = &limatype.LimaYAML{}
		if err := limayaml.Unmarshal(tmpl.Bytes, tmpl.Config, tmpl.Locator); err != nil {
			tmpl.Config = nil
			return err
		}
	}
	return nil
}
