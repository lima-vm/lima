// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limatmpl

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

type useAbsLocatorsTestCase struct {
	description string
	locator     string
	template    string
	expected    string
}

var useAbsLocatorsTestCases = []useAbsLocatorsTestCase{
	{
		"Template without base or script file",
		"template://foo",
		`arch: aarch64`,
		`arch: aarch64`,
	},
	{
		"Single string base template",
		"template://foo",
		`base: bar.yaml`,
		`base: template://bar.yaml`,
	},
	{
		"Flow style array of one base template",
		"template://foo",
		`base: {url: bar.yaml, digest: deadbeef}`,
		// not sure why the quotes around the URL were added; maybe because we don't copy the style from the source
		`base: {url: 'template://bar.yaml', digest: deadbeef}`,
	},
	{
		"Flow style array of sequence of two base URLs",
		"template://foo",
		`base: [bar.yaml, baz.yaml]`,
		`base: ['template://bar.yaml', 'template://baz.yaml']`,
	},
	{
		"Flow style array of sequence of two base locator objects",
		"template://foo",
		`base: [{url: bar.yaml, digest: deadbeef}, {url: baz.yaml, digest: decafbad}]`,
		`base: [{url: 'template://bar.yaml', digest: deadbeef}, {url: 'template://baz.yaml', digest: decafbad}]`,
	},
	{
		"Block style array of one base template",
		"template://foo",
		`
base:
- bar.yaml
`,
		`
base:
- template://bar.yaml`,
	},
	{
		"Block style of four base templates",
		"template://foo",
		`
base:
- bar.yaml
- template://my
- https://example.com/my.yaml
- baz.yaml
`,
		`
base:
- template://bar.yaml
- template://my
- https://example.com/my.yaml
- template://baz.yaml
`,
	},
	{
		"Provisioning and probe scripts",
		"template://experimental/foo",
		`
provision:
- mode: user
  file: userscript.sh
- mode: system
  file:
    url: systemscript.sh
    digest: abc123
probes:
- file: probe.sh
- file:
    url: probe.sh
    digest: digest
`,
		`
provision:
- mode: user
  file: template://experimental/userscript.sh
- mode: system
  file:
    url: template://experimental/systemscript.sh
    digest: abc123
probes:
- file: template://experimental/probe.sh
- file:
    url: template://experimental/probe.sh
    digest: digest
`,
	},
}

func TestUseAbsLocators(t *testing.T) {
	for _, tc := range useAbsLocatorsTestCases {
		t.Run(tc.description, func(t *testing.T) { RunUseAbsLocatorTest(t, tc) })
	}
}

func RunUseAbsLocatorTest(t *testing.T, tc useAbsLocatorsTestCase) {
	tmpl := &Template{
		Bytes:   []byte(strings.TrimSpace(tc.template)),
		Locator: tc.locator,
	}
	err := tmpl.UseAbsLocators()
	assert.NilError(t, err, tc.description)

	actual := strings.TrimSpace(string(tmpl.Bytes))
	expected := strings.TrimSpace(tc.expected)
	assert.Equal(t, actual, expected, tc.description)
}

func TestBasePath(t *testing.T) {
	// On Windows the root will be something like "C:\"
	root, err := filepath.Abs("/")
	assert.NilError(t, err)
	volume := filepath.VolumeName(root)

	t.Run("", func(t *testing.T) {
		actual, err := basePath("/foo")
		assert.NilError(t, err)
		assert.Equal(t, actual, root)
	})

	t.Run("", func(t *testing.T) {
		actual, err := basePath("/foo/bar")
		assert.NilError(t, err)
		assert.Equal(t, actual, filepath.Clean(volume+"/foo"))
	})

	t.Run("", func(t *testing.T) {
		actual, err := basePath("template://foo")
		assert.NilError(t, err)
		assert.Equal(t, actual, "template://")
	})

	t.Run("", func(t *testing.T) {
		actual, err := basePath("template://foo/bar")
		assert.NilError(t, err)
		assert.Equal(t, actual, "template://foo")
	})

	t.Run("", func(t *testing.T) {
		actual, err := basePath("http://host/foo")
		assert.NilError(t, err)
		assert.Equal(t, actual, "http://host")
	})

	t.Run("", func(t *testing.T) {
		actual, err := basePath("http://host/foo/bar")
		assert.NilError(t, err)
		assert.Equal(t, actual, "http://host/foo")
	})

	t.Run("", func(t *testing.T) {
		actual, err := basePath("file:///foo")
		assert.NilError(t, err)
		assert.Equal(t, actual, "file:///")
	})

	t.Run("", func(t *testing.T) {
		actual, err := basePath("file:///foo/bar")
		assert.NilError(t, err)
		assert.Equal(t, actual, "file:///foo")
	})
}

func TestAbsPath(t *testing.T) {
	root, err := filepath.Abs("/")
	assert.NilError(t, err)
	volume := filepath.VolumeName(root)

	t.Run("If the locator is already an absolute path, it is returned unchanged", func(t *testing.T) {
		actual, err := absPath(volume+"/foo", volume+"/root")
		assert.NilError(t, err)
		assert.Equal(t, actual, filepath.Clean(volume+"/foo"))
	})

	t.Run("If the locator is a rooted path without volume name, then the volume will be added", func(t *testing.T) {
		actual, err := absPath("/foo", volume+"/root")
		assert.NilError(t, err)
		assert.Equal(t, actual, filepath.Clean(volume+"/foo"))
	})

	t.Run("", func(t *testing.T) {
		actual, err := absPath("template://foo", volume+"/root")
		assert.NilError(t, err)
		assert.Equal(t, actual, "template://foo")
	})

	t.Run("", func(t *testing.T) {
		actual, err := absPath("http://host/foo", volume+"/root")
		assert.NilError(t, err)
		assert.Equal(t, actual, "http://host/foo")
	})

	t.Run("", func(t *testing.T) {
		actual, err := absPath("file:///foo", volume+"/root")
		assert.NilError(t, err)
		assert.Equal(t, actual, "file:///foo")
	})

	t.Run("Can't have relative path when reading from STDIN", func(t *testing.T) {
		_, err = absPath("foo", "-")
		assert.ErrorContains(t, err, "STDIN")
	})

	t.Run("Relative paths must be underneath the basePath", func(t *testing.T) {
		_, err = absPath("../foo", volume+"/root")
		assert.ErrorContains(t, err, "'../'")
	})

	t.Run("locator must not be empty", func(t *testing.T) {
		_, err = absPath("", "foo")
		assert.ErrorContains(t, err, "locator is empty")
	})

	t.Run("basePath must not be empty", func(t *testing.T) {
		_, err = absPath("foo", "")
		assert.ErrorContains(t, err, "basePath is empty")
	})

	t.Run("", func(t *testing.T) {
		_, err = absPath("./foo", "")
		assert.ErrorContains(t, err, "empty")
	})

	t.Run("", func(t *testing.T) {
		actual, err := absPath("./foo", volume+"/root")
		assert.NilError(t, err)
		assert.Equal(t, actual, filepath.Clean(volume+"/root/foo"))
	})

	if runtime.GOOS == "windows" {
		t.Run("Relative locators must not include volume names", func(t *testing.T) {
			_, err := absPath(volume+"foo", volume+"/root")
			assert.ErrorContains(t, err, "volume")
		})
	}

	t.Run("", func(t *testing.T) {
		actual, err := absPath("foo", "template://")
		assert.NilError(t, err)
		assert.Equal(t, actual, "template://foo")
	})

	t.Run("", func(t *testing.T) {
		actual, err := absPath("bar", "template://foo")
		assert.NilError(t, err)
		assert.Equal(t, actual, "template://foo/bar")
	})

	t.Run("", func(t *testing.T) {
		actual, err := absPath("foo", "http://host")
		assert.NilError(t, err)
		assert.Equal(t, actual, "http://host/foo")
	})

	t.Run("", func(t *testing.T) {
		actual, err := absPath("bar", "http://host/foo")
		assert.NilError(t, err)
		assert.Equal(t, actual, "http://host/foo/bar")
	})

	t.Run("", func(t *testing.T) {
		actual, err := absPath("foo", "file:///")
		assert.NilError(t, err)
		assert.Equal(t, actual, "file:///foo")
	})

	t.Run("", func(t *testing.T) {
		actual, err := absPath("bar", "file:///foo")
		assert.NilError(t, err)
		assert.Equal(t, actual, "file:///foo/bar")
	})
}
