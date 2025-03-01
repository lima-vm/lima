// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/lima-vm/lima/pkg/yqutil"
	"github.com/sirupsen/logrus"
)

const (
	documentStart = "---\n"
	documentEnd   = "...\n"
)

// Marshal the struct as a YAML document, optionally as a stream.
func Marshal(y *LimaYAML, stream bool) ([]byte, error) {
	b, err := yaml.Marshal(y)
	if err != nil {
		return nil, err
	}
	if stream {
		doc := documentStart + string(b) + documentEnd
		b = []byte(doc)
	}
	return b, nil
}

func unmarshalDisk(dst *Disk, b []byte) error {
	var s string
	if err := yaml.Unmarshal(b, &s); err == nil {
		*dst = Disk{Name: s}
		return nil
	}
	return yaml.Unmarshal(b, dst)
}

func Unmarshal(data []byte, y *LimaYAML, comment string) error {
	if err := yaml.UnmarshalWithOptions(data, y, yaml.CustomUnmarshaler[Disk](unmarshalDisk)); err != nil {
		return fmt.Errorf("failed to unmarshal YAML (%s): %w", comment, err)
	}
	// the go-yaml library doesn't catch all markup errors, unfortunately
	// make sure to get a "second opinion", using the same library as "yq"
	if err := yqutil.ValidateContent(data); err != nil {
		return fmt.Errorf("failed to unmarshal YAML (%s): %w", comment, err)
	}
	var ignore LimaYAML
	if err := yaml.UnmarshalWithOptions(data, &ignore, yaml.Strict(), yaml.CustomUnmarshaler[Disk](unmarshalDisk)); err != nil {
		logrus.WithField("comment", comment).WithError(err).Warn("Non-strict YAML detected; please check for typos")
	}
	return nil
}
