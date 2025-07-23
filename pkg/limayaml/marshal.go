// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/yqutil"
)

const (
	documentStart = "---\n"
	documentEnd   = "...\n"
)

// Marshal the struct as a YAML document, optionally as a stream.
func Marshal(y *limatype.LimaYAML, stream bool) ([]byte, error) {
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

func unmarshalDisk(dst *limatype.Disk, b []byte) error {
	var s string
	if err := yaml.Unmarshal(b, &s); err == nil {
		*dst = limatype.Disk{Name: s}
		return nil
	}
	return yaml.Unmarshal(b, dst)
}

// unmarshalBaseTemplates unmarshalls `base` which is either a string or a list of Locators.
func unmarshalBaseTemplates(dst *limatype.BaseTemplates, b []byte) error {
	var s string
	if err := yaml.Unmarshal(b, &s); err == nil {
		*dst = limatype.BaseTemplates{limatype.LocatorWithDigest{URL: s}}
		return nil
	}
	return yaml.UnmarshalWithOptions(b, dst, yaml.CustomUnmarshaler[limatype.LocatorWithDigest](unmarshalLocatorWithDigest))
}

// unmarshalLocator unmarshalls a locator which is either a string or a Locator struct.
func unmarshalLocatorWithDigest(dst *limatype.LocatorWithDigest, b []byte) error {
	var s string
	if err := yaml.Unmarshal(b, &s); err == nil {
		*dst = limatype.LocatorWithDigest{URL: s}
		return nil
	}
	return yaml.Unmarshal(b, dst)
}

func Unmarshal(data []byte, y *limatype.LimaYAML, comment string) error {
	opts := []yaml.DecodeOption{
		yaml.CustomUnmarshaler[limatype.BaseTemplates](unmarshalBaseTemplates),
		yaml.CustomUnmarshaler[limatype.Disk](unmarshalDisk),
		yaml.CustomUnmarshaler[limatype.LocatorWithDigest](unmarshalLocatorWithDigest),
	}
	if err := yaml.UnmarshalWithOptions(data, y, opts...); err != nil {
		return fmt.Errorf("failed to unmarshal YAML (%s): %w", comment, err)
	}
	// The go-yaml library doesn't catch all markup errors, unfortunately
	// make sure to get a "second opinion", using the same library as "yq"
	if err := yqutil.ValidateContent(data); err != nil {
		return fmt.Errorf("failed to unmarshal YAML (%s): %w", comment, err)
	}
	// Finally log a warning if the YAML file violates the "strict" rules
	opts = append(opts, yaml.Strict())
	var ignore limatype.LimaYAML
	if err := yaml.UnmarshalWithOptions(data, &ignore, opts...); err != nil {
		logrus.WithField("comment", comment).WithError(err).Warn("Non-strict YAML detected; please check for typos")
	}
	return nil
}
