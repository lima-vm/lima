// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

type instanceAdapter struct {
	i *limatype.Instance
}

func (a instanceAdapter) Field(fieldpath []string) (string, bool) {
	if len(fieldpath) == 0 {
		return "", false
	}

	fieldName := fieldpath[0]
	v := reflect.ValueOf(a.i).Elem()
	t := v.Type()

	// Special handling for "param" map
	if fieldName == "param" {
		if len(fieldpath) == 2 {
			paramField := v.FieldByName("Param")
			if paramField.IsValid() && !paramField.IsNil() && paramField.Kind() == reflect.Map {
				val := paramField.MapIndex(reflect.ValueOf(fieldpath[1]))
				if val.IsValid() {
					return val.String(), true
				}
			}
		}
		return "", false
	}

	// Iterate over fields to find matching JSON tag
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		if tag == "" {
			continue
		}
		tagName, _, _ := strings.Cut(tag, ",")
		if tagName == fieldName {
			return valToString(v.Field(i)), true
		}
	}

	return "", false
}

func valToString(v reflect.Value) string {
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	default:
		return fmt.Sprintf("%v", v.Interface())
	}
}