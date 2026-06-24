// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
)

func TestSlotSet(t *testing.T) {
	s := newSlotSet(8)
	seen := map[int]bool{}
	for i := 0; i < 8; i++ {
		slot, ok := s.allocate()
		assert.Assert(t, ok, "allocation %d should succeed", i)
		assert.Assert(t, !seen[slot], "slot %d allocated twice", slot)
		seen[slot] = true
	}
	if _, ok := s.allocate(); ok {
		t.Error("9th allocation should fail")
	}
	s.release(3)
	slot, ok := s.allocate()
	assert.Assert(t, ok)
	assert.Equal(t, slot, 3)
}

func TestBuild9pFsdevAddCmd(t *testing.T) {
	assert.Equal(t,
		build9pFsdevAddCmd("fsdev-hp-2", "/home/me/code", "none", false),
		"fsdev_add local,id=fsdev-hp-2,path=/home/me/code,security_model=none,readonly=on")
	assert.Equal(t,
		build9pFsdevAddCmd("fsdev-hp-2", "/home/me/code", "none", true),
		"fsdev_add local,id=fsdev-hp-2,path=/home/me/code,security_model=none")
}

func TestBuildDeviceAddJSON(t *testing.T) {
	b, err := buildExecuteJSON("device_add", map[string]any{
		"driver": "vhost-user-fs-pci", "id": "lima-fs-2", "chardev": "char-fs-hp-2",
		"tag": "lima-abc", "bus": "lima-hp-2", "queue-size": 1024,
	})
	assert.NilError(t, err)
	var got map[string]any
	assert.NilError(t, json.Unmarshal(b, &got))
	assert.Equal(t, got["execute"], "device_add")
	args := got["arguments"].(map[string]any)
	assert.Equal(t, args["driver"], "vhost-user-fs-pci")
	assert.Equal(t, args["tag"], "lima-abc")
	assert.Equal(t, args["bus"], "lima-hp-2")
}

func TestBuildChardevAddJSON(t *testing.T) {
	b, err := buildChardevAddJSON("char-fs-hp-1", "/run/lima/v.sock")
	assert.NilError(t, err)
	var got map[string]any
	assert.NilError(t, json.Unmarshal(b, &got))
	assert.Equal(t, got["execute"], "chardev-add")
	args := got["arguments"].(map[string]any)
	assert.Equal(t, args["id"], "char-fs-hp-1")
	backend := args["backend"].(map[string]any)
	assert.Equal(t, backend["type"], "socket")
}
