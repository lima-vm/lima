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

	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/usrlocalsharelima"
)

type Template struct {
	Name     string `json:"name"`
	Location string `json:"location"`
}

func templatesPaths() ([]string, error) {
	if tmplPath := os.Getenv("LIMA_TEMPLATES_PATH"); tmplPath != "" {
		return strings.Split(tmplPath, string(filepath.ListSeparator)), nil
	}
	limaTemplatesDir, err := dirnames.LimaTemplatesDir()
	if err != nil {
		return nil, err
	}
	shareDir, err := usrlocalsharelima.Dir()
	if err != nil {
		return nil, err
	}
	return []string{
		limaTemplatesDir,
		filepath.Join(shareDir, "templates"),
	}, nil
}

// Read searches for template `name` in all template directories and returns the
// contents of the first one found. Template names cannot contain the substring ".."
// to make sure they don't reference files outside the template directories. We are
// not using securejoin.SecureJoin because the actual template may be a symlink to a
// directory elsewhere (e.g. when installed by Homebrew).
func Read(name string) ([]byte, error) {
	doubleDot := ".."
	if strings.Contains(name, doubleDot) {
		return nil, fmt.Errorf("template name %q must not contain %q", name, doubleDot)
	}
	paths, err := templatesPaths()
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
		// Normalize filePath for error messages because template names always use forward slashes
		filePath := filepath.Clean(filepath.Join(templatesDir, name))
		if b, err := os.ReadFile(filePath); !errors.Is(err, os.ErrNotExist) {
			return b, err
		}
	}
	return nil, fmt.Errorf("template %q not found", name)
}

const Default = "default"

// Templates returns a list of Template structures containing the Name and Location for each template.
// It searches all template directories, but only the first template of a given name is recorded.
// Only non-hidden files with a ".yaml" file extension are considered templates.
// The final result is sorted alphabetically by template name.
func Templates() ([]Template, error) {
	paths, err := templatesPaths()
	if err != nil {
		return nil, err
	}

	templates := make(map[string]string)
	for _, templatesDir := range paths {
		if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
			continue
		}
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
