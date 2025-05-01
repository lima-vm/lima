// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package templatestore

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/lima-vm/lima/pkg/usrlocalsharelima"
)

type Template struct {
	Name     string `json:"name"`
	Location string `json:"location"`
}

func TemplatesPaths() ([]string, error) {
	var paths []string
	if tmplPath := os.Getenv("LIMA_TEMPLATES_PATH"); tmplPath != "" {
		paths = strings.Split(tmplPath, string(filepath.ListSeparator))
	} else {
		dir, err := usrlocalsharelima.Dir()
		if err != nil {
			return nil, err
		}
		paths = []string{filepath.Join(dir, "templates")}
	}
	return paths, nil
}

func Read(name string) ([]byte, error) {
	paths, err := TemplatesPaths()
	if err != nil {
		return nil, err
	}
	ext := filepath.Ext(name)
	// Append .yaml extension if name doesn't have an extension, or if it starts with a digit.
	// So "docker.sh" would remain unchanged but "ubuntu-24.04" becomes "ubuntu-24.04.yaml".
	if len(ext) < 2 || unicode.IsDigit(rune(ext[1])) {
		name += ".yaml"
	}
	for _, templatesDir := range paths {
		filePath, err := securejoin.SecureJoin(templatesDir, name)
		if err != nil {
			return nil, err
		}
		if b, err := os.ReadFile(filePath); !errors.Is(err, os.ErrNotExist) {
			return b, err
		}
	}
	return nil, fmt.Errorf("template %q not found", name)
}

const Default = "default"

func Templates() ([]Template, error) {
	paths, err := TemplatesPaths()
	if err != nil {
		return nil, err
	}

	templates := make(map[string]string)
	for _, templatesDir := range paths {
		walkDirFn := func(p string, _ fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			base := filepath.Base(p)
			if strings.HasPrefix(base, ".") || !strings.HasSuffix(base, ".yaml") {
				return nil
			}
			// Name is like "default", "debian", "deprecated/centos-7", ...
			name := strings.TrimSuffix(strings.TrimPrefix(p, templatesDir+"/"), ".yaml")
			if _, ok := templates[name]; !ok {
				templates[name] = p
			}
			return nil
		}
		if err = filepath.WalkDir(templatesDir, walkDirFn); err != nil {
			return nil, err
		}
	}
	var res []Template
	for name, loc := range templates {
		res = append(res, Template{Name: name, Location: loc})
	}
	slices.SortFunc(res, func(i, j Template) int { return cmp.Compare(i.Name, j.Name) })
	return res, nil
}
