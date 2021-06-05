package cidata

import (
	_ "embed"
	"path/filepath"

	"github.com/AkihiroSuda/lima/pkg/limayaml"

	"github.com/AkihiroSuda/lima/pkg/templateutil"
	"github.com/containerd/containerd/identifiers"
	"github.com/pkg/errors"
)

var (
	//go:embed user-data.TEMPLATE
	userDataTemplate string
	//go:embed meta-data.TEMPLATE
	metaDataTemplate string
)

type TemplateArgs struct {
	Name       string // instance name
	User       string // user name
	UID        int
	SSHPubKeys []string
	Mounts     []string // abs path, accessible by the User
	Provision  []limayaml.Provision
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

func GenerateUserData(args TemplateArgs) ([]byte, error) {
	if err := ValidateTemplateArgs(args); err != nil {
		return nil, err
	}
	return templateutil.Execute(userDataTemplate, args)
}

func GenerateMetaData(args TemplateArgs) ([]byte, error) {
	if err := ValidateTemplateArgs(args); err != nil {
		return nil, err
	}
	return templateutil.Execute(metaDataTemplate, args)
}
