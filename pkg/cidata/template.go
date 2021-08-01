package cidata

import (
	"bytes"
	"embed"
	_ "embed"
	"io/fs"
	"path/filepath"

	"github.com/AkihiroSuda/lima/pkg/iso9660util"

	"github.com/AkihiroSuda/lima/pkg/templateutil"
	"github.com/containerd/containerd/identifiers"
	"github.com/pkg/errors"
)

//go:embed cidata.TEMPLATE.d
var templateFS embed.FS

const templateFSRoot = "cidata.TEMPLATE.d"

type Containerd struct {
	System bool
	User   bool
}
type Network struct {
	MACAddress string
	Name       string
}
type TemplateArgs struct {
	Name       string // instance name
	User       string // user name
	UID        int
	SSHPubKeys []string
	Mounts     []string // abs path, accessible by the User
	Containerd Containerd
	Networks   []Network
}

func ValidateTemplateArgs(args TemplateArgs) error {
	if err := identifiers.Validate(args.Name); err != nil {
		return err
	}
	if err := identifiers.Validate(args.User); err != nil {
		return err
	}
	if args.User == "root" {
		return errors.New("field User must not be \"root\"")
	}
	if args.UID == 0 {
		return errors.New("field UID must not be 0")
	}
	if len(args.SSHPubKeys) == 0 {
		return errors.New("field SSHPubKeys must be set")
	}
	for i, f := range args.Mounts {
		if !filepath.IsAbs(f) {
			return errors.Errorf("field mounts[%d] must be absolute, got %q", i, f)
		}
	}
	return nil
}

func ExecuteTemplate(args TemplateArgs) ([]iso9660util.Entry, error) {
	if err := ValidateTemplateArgs(args); err != nil {
		return nil, err
	}

	fsys, err := fs.Sub(templateFS, templateFSRoot)
	if err != nil {
		return nil, err
	}

	var layout []iso9660util.Entry
	walkFn := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return errors.Errorf("got non-regular file %q", path)
		}
		templateB, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		b, err := templateutil.Execute(string(templateB), args)
		if err != nil {
			return err
		}
		layout = append(layout, iso9660util.Entry{
			Path:   path,
			Reader: bytes.NewReader(b),
		})
		return nil
	}

	if err := fs.WalkDir(fsys, ".", walkFn); err != nil {
		return nil, err
	}

	return layout, nil
}
