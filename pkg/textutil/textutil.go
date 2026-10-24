// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package textutil

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"unicode/utf8"

	"github.com/goccy/go-yaml"
)

// ExecuteTemplate executes a text/template template with TemplateFuncMap registered,
// so templates such as the launchd plists can escape dynamic values with the "xml" function.
func ExecuteTemplate(tmpl string, args any) ([]byte, error) {
	x, err := template.New("").Funcs(TemplateFuncMap).Parse(tmpl)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := x.Execute(&b, args); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// PrefixString adds prefix to beginning of each line.
func PrefixString(prefix, text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

// IndentString add spaces to beginning of each line.
func IndentString(size int, text string) string {
	prefix := strings.Repeat(" ", size)
	return PrefixString(prefix, text)
}

// MissingString returns message if the text is empty.
func MissingString(message, text string) string {
	if text == "" {
		return message
	}
	return text
}

// TemplateFuncMap is a text/template FuncMap.
var TemplateFuncMap = template.FuncMap{
	"json": func(v any) string {
		var b bytes.Buffer
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(v); err != nil {
			panic(fmt.Errorf("failed to marshal as JSON: %+v: %w", v, err))
		}
		return strings.TrimSuffix(b.String(), "\n")
	},
	"yaml": func(v any) string {
		var b bytes.Buffer
		enc := yaml.NewEncoder(&b)
		if err := enc.Encode(v); err != nil {
			panic(fmt.Errorf("failed to marshal as YAML: %+v: %w", v, err))
		}
		return "---\n" + strings.TrimSuffix(b.String(), "\n")
	},
	"indent": func(a ...any) (string, error) {
		if len(a) == 0 {
			return "", errors.New("function takes at least one string argument")
		}
		if len(a) > 2 {
			return "", errors.New("function takes at most 2 arguments")
		}
		var ok bool
		size := 2
		if len(a) > 1 {
			if size, ok = a[0].(int); !ok {
				return "", errors.New("optional first argument must be an integer")
			}
		}
		text := ""
		if text, ok = a[len(a)-1].(string); !ok {
			return "", errors.New("last argument must be a string")
		}
		return IndentString(size, text), nil
	},
	"missing": func(a ...any) (string, error) {
		if len(a) == 0 {
			return "", errors.New("function takes at least one string argument")
		}
		if len(a) > 2 {
			return "", errors.New("function takes at most 2 arguments")
		}
		var ok bool
		message := "<missing>"
		if len(a) > 1 {
			if message, ok = a[0].(string); !ok {
				return "", errors.New("optional first argument must be a string")
			}
		}
		text := ""
		if text, ok = a[len(a)-1].(string); !ok {
			return "", errors.New("last argument must be a string")
		}
		return MissingString(message, text), nil
	},
	"xml": func(s string) (string, error) {
		// Reject anything XML 1.0 cannot represent before escaping. utf8.ValidString alone is not
		// enough: xml.EscapeText silently rewrites a valid-UTF-8 but non-XML rune (such as a control
		// character like U+0001) to U+FFFD without returning an error, which would quietly change a
		// value such as a path. Fail loudly instead.
		if !utf8.ValidString(s) {
			return "", fmt.Errorf("value is not valid UTF-8: %q", s)
		}
		for _, r := range s {
			switch {
			case r == '\t' || r == '\n' || r == '\r':
			case r >= 0x20 && r <= 0xD7FF:
			case r >= 0xE000 && r <= 0xFFFD:
			case r >= 0x10000 && r <= 0x10FFFF:
			default:
				return "", fmt.Errorf("value contains character U+%04X not allowed in XML 1.0: %q", r, s)
			}
		}
		var b bytes.Buffer
		if err := xml.EscapeText(&b, []byte(s)); err != nil {
			return "", err
		}
		return b.String(), nil
	},
}

// FuncHelp is help for TemplateFuncMap.
var FuncHelp = []string{
	"indent <size>: add spaces to beginning of each line",
	"missing <message>: return message if the text is empty",
	"xml <value>: escape a value for XML, rejecting characters XML 1.0 cannot represent",
}
