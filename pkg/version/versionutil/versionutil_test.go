// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package versionutil

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestGreaterThan(t *testing.T) {
	assert.Equal(t, GreaterThan("", "0.1.0"), false)
	assert.Equal(t, GreaterThan("0.0.1", "0.1.0"), false)
	assert.Equal(t, GreaterThan("0.1.0", "0.1.0"), false)
	assert.Equal(t, GreaterThan("0.1.0-2", "0.1.0"), true)
	assert.Equal(t, GreaterThan("0.2.0", "0.1.0"), true)
	assert.Equal(t, GreaterThan("abacab", "0.1.0"), true)
}

func TestGreaterEqual(t *testing.T) {
	assert.Equal(t, GreaterEqual("", ""), true)
	assert.Equal(t, GreaterEqual("", "0.1.0"), false)
	assert.Equal(t, GreaterEqual("0.0.1", "0.1.0"), false)
	assert.Equal(t, GreaterEqual("0.1.0", "0.1.0"), true)
	assert.Equal(t, GreaterEqual("0.1.0-2", "0.1.0"), true)
	assert.Equal(t, GreaterEqual("0.2.0", "0.1.0"), true)
	assert.Equal(t, GreaterEqual("abacab", "0.1.0"), true)
}

func TestParse(t *testing.T) {
	v1, err1 := Parse("v0.19.1-16-gf3dc6ed.m")
	assert.NilError(t, err1)
	assert.Equal(t, v1.Major, int64(0))
	assert.Equal(t, v1.Minor, int64(19))
	assert.Equal(t, v1.Patch, int64(1))

	v2, err2 := Parse("v0.19.1.m")
	assert.NilError(t, err2)
	assert.Equal(t, v2.Major, int64(0))
	assert.Equal(t, v2.Minor, int64(19))
	assert.Equal(t, v2.Patch, int64(1))
}
