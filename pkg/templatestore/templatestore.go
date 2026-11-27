// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package templatestore

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/usrlocal"
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
	shareLimaDirs, err := usrlocal.ShareLima()
	if err != nil {
		return nil, err
	}
	res := []string{limaTemplatesDir}
	for _, shareLimaDir := range shareLimaDirs {
		res = append(res, filepath.Join(shareLimaDir, "templates"))
	}
	return res, nil
}

// Layer is one source of templates.
type Layer struct {
	FS  fs.FS
	Dir string // the host directory that FS reads, or "" when the layer is not a directory on disk
}

type layersKey struct{}

// WithLayers returns a context whose template lookups search layers, in order,
// instead of the template directories.
func WithLayers(ctx context.Context, layers []Layer) context.Context {
	return context.WithValue(ctx, layersKey{}, layers)
}

// layers returns the layers set by WithLayers, or else one directory layer per template directory.
func layers(ctx context.Context) ([]Layer, error) {
	if l, ok := ctx.Value(layersKey{}).([]Layer); ok {
		return l, nil
	}
	paths, err := templatesPaths()
	if err != nil {
		return nil, err
	}
	res := make([]Layer, 0, len(paths))
	for _, dir := range paths {
		// os.DirFS opens files through os.Open, so a template symlinked elsewhere (e.g. by Homebrew) still resolves.
		res = append(res, Layer{FS: os.DirFS(dir), Dir: dir})
	}
	return res, nil
}

// Read searches for template `name` in all template layers and returns the
// contents of the first one found. Template names cannot contain the substring ".."
// to make sure they don't reference files outside the template layers. We are
// not using securejoin.SecureJoin because the actual template may be a symlink to a
// directory elsewhere (e.g. when installed by Homebrew).
func Read(ctx context.Context, name string) ([]byte, error) {
	doubleDot := ".."
	if strings.Contains(name, doubleDot) {
		return nil, fmt.Errorf("template name %#q must not contain %#q", name, doubleDot)
	}
	// "template:/foo" reaches here as "/foo", but fs.ValidPath rejects a leading slash.
	name = strings.TrimPrefix(path.Clean(name), "/")
	ext := path.Ext(name)
	// Append .yaml extension if name doesn't have an extension, or if it starts with a digit.
	// So "docker.sh" would remain unchanged but "ubuntu-24.04" becomes "ubuntu-24.04.yaml".
	if len(ext) < 2 || unicode.IsDigit(rune(ext[1])) {
		name += ".yaml"
	}
	layers, err := layers(ctx)
	if err != nil {
		return nil, err
	}
	for _, layer := range layers {
		b, err := fs.ReadFile(layer.FS, name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		// os.DirFS reports the path relative to its root; name the host file instead.
		if pathErr, ok := errors.AsType[*fs.PathError](err); ok && layer.Dir != "" {
			pathErr.Path = filepath.Join(layer.Dir, filepath.FromSlash(name))
		}
		return b, err
	}
	return nil, fmt.Errorf("template %#q not found", name)
}

const Default = "default"

// Templates returns a list of Template structures containing the Name and Location for each template.
// It searches all template layers, but only the first template of a given name is recorded.
// Only non-hidden files with a ".yaml" file extension are considered templates.
// The Location is the host file path for a directory layer, and "template:<name>" otherwise.
// The final result is sorted alphabetically by template name.
func Templates(ctx context.Context) ([]Template, error) {
	layers, err := layers(ctx)
	if err != nil {
		return nil, err
	}

	templates := make(map[string]string)
	for _, layer := range layers {
		if _, err := fs.Stat(layer.FS, "."); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		walkDirFn := func(p string, _ fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			base := path.Base(p)
			if strings.HasPrefix(base, ".") || !strings.HasSuffix(base, ".yaml") {
				return nil
			}
			// Name is like "default", "debian", "deprecated/centos-7", ...
			name := strings.TrimSuffix(p, ".yaml")
			if _, ok := templates[name]; ok {
				return nil
			}
			if layer.Dir != "" {
				templates[name] = filepath.Join(layer.Dir, filepath.FromSlash(p))
			} else {
				templates[name] = "template:" + name
			}
			return nil
		}
		if err = fs.WalkDir(layer.FS, ".", walkDirFn); err != nil {
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
