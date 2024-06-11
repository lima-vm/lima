package templatestore

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/lima-vm/lima/pkg/usrlocalsharelima"
)

type Template struct {
	Name     string `json:"name"`
	Location string `json:"location"`
}

func Read(name string) ([]byte, error) {
	dir, err := usrlocalsharelima.Dir()
	if err != nil {
		return nil, err
	}
	yamlPath, err := securejoin.SecureJoin(filepath.Join(dir, "templates"), name+".yaml")
	if err != nil {
		return nil, err
	}
	return os.ReadFile(yamlPath)
}

const Default = "default"

func Templates() ([]Template, error) {
	usrlocalsharelimaDir, err := usrlocalsharelima.Dir()
	if err != nil {
		return nil, err
	}
	templatesDir := filepath.Join(usrlocalsharelimaDir, "templates")

	var res []Template
	walkDirFn := func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		base := filepath.Base(p)
		if strings.HasPrefix(base, ".") || !strings.HasSuffix(base, ".yaml") {
			return nil
		}
		x := Template{
			// Name is like "default", "debian", "deprecated/centos-7", ...
			Name:     strings.TrimSuffix(strings.TrimPrefix(p, templatesDir+"/"), ".yaml"),
			Location: p,
		}
		res = append(res, x)
		return nil
	}
	if err = filepath.WalkDir(templatesDir, walkDirFn); err != nil {
		return nil, err
	}
	return res, nil
}
