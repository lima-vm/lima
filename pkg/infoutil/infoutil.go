package infoutil

import (
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/templatestore"
	"github.com/lima-vm/lima/pkg/version"
)

type Info struct {
	Version         string                   `json:"version"`
	Templates       []templatestore.Template `json:"templates"`
	DefaultTemplate *limayaml.LimaYAML       `json:"defaultTemplate"`
	LimaHome        string                   `json:"limaHome"`
	// TODO: add diagnostic info of QEMU
}

func GetInfo() (*Info, error) {
	b, err := templatestore.Read(templatestore.Default)
	if err != nil {
		return nil, err
	}
	y, err := limayaml.Load(b, "")
	if err != nil {
		return nil, err
	}
	info := &Info{
		Version:         version.Version,
		DefaultTemplate: y,
	}
	info.Templates, err = templatestore.Templates()
	if err != nil {
		return nil, err
	}
	info.LimaHome, err = dirnames.LimaDir()
	if err != nil {
		return nil, err
	}
	return info, nil
}
