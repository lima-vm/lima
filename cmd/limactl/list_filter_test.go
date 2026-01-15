package main

import (
	"testing"

	"github.com/containerd/containerd/v2/pkg/filters"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"gotest.tools/v3/assert"
)

func TestInstanceAdapter(t *testing.T) {
	inst := &limatype.Instance{
		Name:   "test-instance",
		Status: "Running",
		CPUs:   4,
		Param: map[string]string{
			"foo": "bar",
		},
	}

	adapter := instanceAdapter{i: inst}

	t.Run("Match Name", func(t *testing.T) {
		f, err := filters.Parse("name==test-instance")
		assert.NilError(t, err)
		assert.Assert(t, f.Match(adapter))
	})

	t.Run("Match Status", func(t *testing.T) {
		f, err := filters.Parse("status==Running")
		assert.NilError(t, err)
		assert.Assert(t, f.Match(adapter))
	})

	t.Run("Match CPU", func(t *testing.T) {
		f, err := filters.Parse("cpus==4")
		assert.NilError(t, err)
		assert.Assert(t, f.Match(adapter))
	})

	t.Run("Match Param", func(t *testing.T) {
		f, err := filters.Parse("param.foo==bar")
		assert.NilError(t, err)
		assert.Assert(t, f.Match(adapter))
	})

	t.Run("No Match", func(t *testing.T) {
		f, err := filters.Parse("name==other")
		assert.NilError(t, err)
		assert.Assert(t, !f.Match(adapter))
	})
}
