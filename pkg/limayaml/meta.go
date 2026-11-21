// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"maps"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

// mergeMeta returns base overlaid with override, without modifying either.
// Maps are merged recursively; any other value in override (scalar, list, or null)
// replaces the value in base. Lists are not appended because Lima cannot know
// the semantics of arbitrary user data.
func mergeMeta(base, override limatype.Meta) limatype.Meta {
	if base == nil && override == nil {
		return nil
	}
	return mergeMaps(base, override)
}

func mergeMaps(base, override map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(override))
	maps.Copy(merged, base)
	for k, v := range override {
		baseMap, baseIsMap := merged[k].(map[string]any)
		overrideMap, overrideIsMap := v.(map[string]any)
		if baseIsMap && overrideIsMap {
			merged[k] = mergeMaps(baseMap, overrideMap)
		} else {
			merged[k] = v
		}
	}
	return merged
}
