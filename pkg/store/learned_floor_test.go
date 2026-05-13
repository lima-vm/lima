// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestLearnedFloor_WriteRead(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	err := WriteLearnedFloor(dir, 4*1024*1024*1024, now)
	assert.NilError(t, err)

	v, learnedAt, err := ReadLearnedFloor(dir)
	assert.NilError(t, err)
	assert.Equal(t, v, uint64(4*1024*1024*1024))
	assert.Equal(t, learnedAt.Unix(), now.Unix())
}

func TestLearnedFloor_NotFound(t *testing.T) {
	dir := t.TempDir()
	v, learnedAt, err := ReadLearnedFloor(dir)
	assert.NilError(t, err)
	assert.Equal(t, v, uint64(0))
	assert.Assert(t, learnedAt.IsZero())
}

func TestLearnedFloor_Corrupt(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "learned-floor"), []byte("garbage"), 0o600)
	assert.NilError(t, err)

	v, learnedAt, err := ReadLearnedFloor(dir)
	assert.NilError(t, err)
	assert.Equal(t, v, uint64(0))
	assert.Assert(t, learnedAt.IsZero())
}

func TestLearnedFloor_Overwrite(t *testing.T) {
	dir := t.TempDir()
	t1 := time.Now().Add(-1 * time.Hour)
	err := WriteLearnedFloor(dir, 3*1024*1024*1024, t1)
	assert.NilError(t, err)

	t2 := time.Now()
	err = WriteLearnedFloor(dir, 5*1024*1024*1024, t2)
	assert.NilError(t, err)

	v, learnedAt, err := ReadLearnedFloor(dir)
	assert.NilError(t, err)
	assert.Equal(t, v, uint64(5*1024*1024*1024))
	assert.Equal(t, learnedAt.Unix(), t2.Unix())
}

// --- E10-2: Learned floor with timestamp ---

func TestLearnedFloor_WriteReadWithTimestamp(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	err := WriteLearnedFloor(dir, 4*1024*1024*1024, now)
	assert.NilError(t, err)

	v, learnedAt, err := ReadLearnedFloor(dir)
	assert.NilError(t, err)
	assert.Equal(t, v, uint64(4*1024*1024*1024))
	// Unix timestamp precision: seconds.
	assert.Assert(t, learnedAt.Unix() == now.Unix())
}

func TestLearnedFloor_OldFormatTreatedAsStale(t *testing.T) {
	dir := t.TempDir()
	// Write old format (bare uint64, no timestamp).
	err := os.WriteFile(filepath.Join(dir, "learned-floor"), []byte("4294967296"), 0o600)
	assert.NilError(t, err)

	v, learnedAt, err := ReadLearnedFloor(dir)
	assert.NilError(t, err)
	assert.Equal(t, v, uint64(4294967296))
	// Old format has zero time → treated as stale.
	assert.Assert(t, learnedAt.IsZero())
}

func TestLearnedFloor_NotFoundWithTimestamp(t *testing.T) {
	dir := t.TempDir()
	v, learnedAt, err := ReadLearnedFloor(dir)
	assert.NilError(t, err)
	assert.Equal(t, v, uint64(0))
	assert.Assert(t, learnedAt.IsZero())
}

func TestLearnedFloor_CorruptWithTimestamp(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "learned-floor"), []byte("garbage"), 0o600)
	assert.NilError(t, err)

	v, learnedAt, err := ReadLearnedFloor(dir)
	assert.NilError(t, err)
	assert.Equal(t, v, uint64(0))
	assert.Assert(t, learnedAt.IsZero())
}
