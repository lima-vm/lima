// This file has been adapted from https://github.com/containerd/nerdctl/blob/v1.0.0/pkg/reflectutil/reflectutil.go
/*
   Copyright The containerd Authors.
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

package reflectutil

import (
	"fmt"
	"reflect"
)

func UnknownNonEmptyFields(structOrStructPtr any, knownNames ...string) []string {
	var unknown []string
	knownNamesMap := make(map[string]struct{}, len(knownNames))
	for _, name := range knownNames {
		knownNamesMap[name] = struct{}{}
	}
	origVal := reflect.ValueOf(structOrStructPtr)
	var val reflect.Value
	switch kind := origVal.Kind(); kind {
	case reflect.Ptr:
		val = origVal.Elem()
	case reflect.Struct:
		val = origVal
	default:
		panic(fmt.Errorf("expected Ptr or Struct, got %+v", kind))
	}
	for i := 0; i < val.NumField(); i++ {
		iField := val.Field(i)
		if isEmpty(iField) {
			continue
		}
		iName := val.Type().Field(i).Name
		if _, ok := knownNamesMap[iName]; !ok {
			unknown = append(unknown, iName)
		}
	}
	return unknown
}

func isEmpty(v reflect.Value) bool {
	// NOTE: IsZero returns false for zero-length map and slice
	if v.IsZero() {
		return true
	}
	switch v.Kind() {
	case reflect.Map, reflect.Slice:
		return v.Len() == 0
	}
	return false
}
