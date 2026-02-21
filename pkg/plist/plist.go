// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package plist provides a parser for XML-formatted plist documents.
// Binary plist is not supported.
package plist

import (
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Plist represents a plist document.
type Plist struct {
	Value Value
}

// Value represents a value.
type Value struct {
	Array Array
	Dict  Dict

	String *string
	Data   []byte
	Date   *time.Time

	Boolean *bool
	Real    *float64
	Integer *int64
}

// Array represents an array.
type Array []Value

// Dict represents a dict.
type Dict map[string]Value

func (p *Plist) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			var v Value
			if err := dec.DecodeElement(&v, &t); err != nil {
				return err
			}
			p.Value = v
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

func (v *Value) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	switch start.Name.Local {
	case "array":
		var arr Array
		if err := dec.DecodeElement(&arr, &start); err != nil {
			return err
		}
		v.Array = arr
		return nil
	case "dict":
		var sub Dict
		if err := dec.DecodeElement(&sub, &start); err != nil {
			return err
		}
		v.Dict = sub
		return nil
	case "string":
		var txt string
		if err := dec.DecodeElement(&txt, &start); err != nil {
			return err
		}
		v.String = &txt
		return nil
	case "data":
		var txt string
		if err := dec.DecodeElement(&txt, &start); err != nil {
			return err
		}
		// remove all whitespace/newlines from base64 text
		b64 := strings.Join(strings.Fields(txt), "")
		db, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return fmt.Errorf("invalid base64 data: %w", err)
		}
		v.Data = db
		return nil
	case "date":
		var txt string
		if err := dec.DecodeElement(&txt, &start); err != nil {
			return err
		}
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(txt))
		if err != nil {
			return fmt.Errorf("invalid date value: %w", err)
		}
		v.Date = &t
		return nil
	case "true":
		b := true
		v.Boolean = &b
		// consume tokens until matching end element
		return dec.Skip()
	case "false":
		b := false
		v.Boolean = &b
		// consume tokens until matching end element
		return dec.Skip()
	case "real":
		var txt string
		if err := dec.DecodeElement(&txt, &start); err != nil {
			return err
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(txt), 64)
		if err != nil {
			return fmt.Errorf("invalid real value: %w", err)
		}
		v.Real = &f
		return nil
	case "integer":
		var txt string
		if err := dec.DecodeElement(&txt, &start); err != nil {
			return err
		}
		i, err := strconv.ParseInt(strings.TrimSpace(txt), 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer value: %w", err)
		}
		v.Integer = &i
		return nil

	default:
		return fmt.Errorf("unsupported plist type: %s", start.Name.Local)
	}
}

func (a *Array) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var vals []Value
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				*a = vals
				return nil
			}
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			var v Value
			if err := dec.DecodeElement(&v, &t); err != nil {
				return err
			}
			vals = append(vals, v)
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				*a = vals
				return nil
			}
		}
	}
}

func (d *Dict) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*d = make(map[string]Value)

	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local != "key" {
				return fmt.Errorf("expected <key> element, got <%s>", t.Name.Local)
			}
			var key string
			if err := dec.DecodeElement(&key, &t); err != nil {
				return err
			}
			var vs xml.StartElement
			for {
				vt, err := dec.Token()
				if err != nil {
					return err
				}
				if se, ok := vt.(xml.StartElement); ok {
					vs = se
					break
				}
			}
			var v Value
			if err := dec.DecodeElement(&v, &vs); err != nil {
				return err
			}
			(*d)[key] = v
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}
