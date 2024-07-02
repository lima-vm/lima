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
