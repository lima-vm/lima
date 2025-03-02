/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
