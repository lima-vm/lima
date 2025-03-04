// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limatmpl

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// UseAbsLocators will replace all relative template locators with absolute ones, so this template
// can be stored anywhere and still reference the same base templates and files.
func (tmpl *Template) UseAbsLocators() error {
	err := tmpl.useAbsLocators()
	return tmpl.ClearOnError(err)
}

func (tmpl *Template) useAbsLocators() error {
	if err := tmpl.Unmarshal(); err != nil {
		return err
	}
	basePath, err := basePath(tmpl.Locator)
	if err != nil {
		return err
	}
	for i, baseLocator := range tmpl.Config.Base {
		absLocator, err := absPath(baseLocator.URL, basePath)
		if err != nil {
			return err
		}
		if i == 0 {
			// base can be either a single string (URL), or a single locator object, or a list whose first element can be either a string or an object
			tmpl.expr.WriteString(fmt.Sprintf("| ($a.base | select(type == \"!!str\")) |= %q\n", absLocator))
			tmpl.expr.WriteString(fmt.Sprintf("| ($a.base | select(type == \"!!map\") | .url) |= %q\n", absLocator))
			tmpl.expr.WriteString(fmt.Sprintf("| ($a.base | select(type == \"!!seq\" and (.[0] | type) == \"!!str\") | .[0]) |= %q\n", absLocator))
			tmpl.expr.WriteString(fmt.Sprintf("| ($a.base | select(type == \"!!seq\" and (.[0] | type) == \"!!map\") | .[0].url) |= %q\n", absLocator))
		} else {
			tmpl.expr.WriteString(fmt.Sprintf("| ($a.base[%d] | select(type == \"!!str\")) |= %q\n", i, absLocator))
			tmpl.expr.WriteString(fmt.Sprintf("| ($a.base[%d] | select(type == \"!!map\") | .url) |= %q\n", i, absLocator))
		}
	}
	for i, p := range tmpl.Config.Probes {
		if p.File != nil {
			absLocator, err := absPath(p.File.URL, basePath)
			if err != nil {
				return err
			}
			tmpl.expr.WriteString(fmt.Sprintf("| ($a.probes[%d].file | select(type == \"!!str\")) = %q\n", i, absLocator))
			tmpl.expr.WriteString(fmt.Sprintf("| ($a.probes[%d].file | select(type == \"!!map\") | .url) = %q\n", i, absLocator))
		}
	}
	for i, p := range tmpl.Config.Provision {
		if p.File != nil {
			absLocator, err := absPath(p.File.URL, basePath)
			if err != nil {
				return err
			}
			tmpl.expr.WriteString(fmt.Sprintf("| ($a.provision[%d].file | select(type == \"!!str\")) = %q\n", i, absLocator))
			tmpl.expr.WriteString(fmt.Sprintf("| ($a.provision[%d].file | select(type == \"!!map\") | .url) = %q\n", i, absLocator))
		}
	}
	return tmpl.evalExpr()
}

// withVolume adds the volume name of the current working directory to a path without volume name.
// On Windows filepath.Abs() only returns a "rooted" name, but does not add the volume name.
// withVolume also normalizes all path separators to the platform native one.
func withVolume(path string) (string, error) {
	if runtime.GOOS == "windows" && filepath.VolumeName(path) == "" {
		root, err := filepath.Abs("/")
		if err != nil {
			return "", err
		}
		path = filepath.VolumeName(root) + path
	}
	return filepath.Clean(path), nil
}

// basePath returns the locator in absolute format, but without the filename part.
func basePath(locator string) (string, error) {
	u, err := url.Parse(locator)
	// Single-letter schemes will be drive names on Windows, e.g. "c:/foo"
	if err == nil && len(u.Scheme) > 1 {
		// path.Dir("") returns ".", which must be removed for url.JoinPath() to do the right thing later
		return u.Scheme + "://" + strings.TrimSuffix(path.Dir(path.Join(u.Host, u.Path)), "."), nil
	}
	base, err := filepath.Abs(filepath.Dir(locator))
	if err != nil {
		return "", err
	}
	return withVolume(base)
}

// absPath either returns the locator directly, or combines it with the basePath if the locator is a relative path.
func absPath(locator, basePath string) (string, error) {
	if locator == "" {
		return "", errors.New("locator is empty")
	}
	u, err := url.Parse(locator)
	if err == nil && len(u.Scheme) > 1 {
		return locator, nil
	}
	// Check for rooted locator; filepath.IsAbs() returns false on Windows when the volume name is missing
	volumeLen := len(filepath.VolumeName(locator))
	if locator[volumeLen] != '/' && locator[volumeLen] != filepath.Separator {
		switch {
		case basePath == "":
			return "", errors.New("basePath is empty")
		case basePath == "-":
			return "", errors.New("can't use relative paths when reading template from STDIN")
		case strings.Contains(locator, "../"):
			return "", fmt.Errorf("relative locator path %q must not contain '../' segments", locator)
		case volumeLen != 0:
			return "", fmt.Errorf("relative locator path %q must not include a volume name", locator)
		}
		u, err = url.Parse(basePath)
		if err != nil {
			return "", err
		}
		if len(u.Scheme) > 1 {
			return u.JoinPath(locator).String(), nil
		}
		locator = filepath.Join(basePath, locator)
	}
	return withVolume(locator)
}
