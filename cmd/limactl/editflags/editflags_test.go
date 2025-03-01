// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package editflags

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestCompleteCPUs(t *testing.T) {
	assert.DeepEqual(t, []int{1}, completeCPUs(1))
	assert.DeepEqual(t, []int{1, 2}, completeCPUs(2))
	assert.DeepEqual(t, []int{1, 2, 4, 8}, completeCPUs(8))
	assert.DeepEqual(t, []int{1, 2, 4, 8, 16, 20}, completeCPUs(20))
}

func TestCompleteMemoryGiB(t *testing.T) {
	assert.DeepEqual(t, []float32{0.5}, completeMemoryGiB(1<<30))
	assert.DeepEqual(t, []float32{1}, completeMemoryGiB(2<<30))
	assert.DeepEqual(t, []float32{1, 2}, completeMemoryGiB(4<<30))
	assert.DeepEqual(t, []float32{1, 2, 4}, completeMemoryGiB(8<<30))
	assert.DeepEqual(t, []float32{1, 2, 4, 8, 10}, completeMemoryGiB(20<<30))
}
