package main

import (
	"fmt"
	"sort"

	"github.com/reproducible-containers/repro-get/pkg/distro"
	"github.com/reproducible-containers/repro-get/pkg/distro/alpine"
	"github.com/reproducible-containers/repro-get/pkg/distro/arch"
	"github.com/reproducible-containers/repro-get/pkg/distro/debian"
	"github.com/reproducible-containers/repro-get/pkg/distro/distroutil/detect"
	"github.com/reproducible-containers/repro-get/pkg/distro/fedora"
	"github.com/reproducible-containers/repro-get/pkg/distro/none"
	"github.com/reproducible-containers/repro-get/pkg/distro/ubuntu"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var knownDistros = map[string]distro.Distro{
	none.Name:   none.New(),
	debian.Name: debian.New(),
	ubuntu.Name: ubuntu.New(),
	fedora.Name: fedora.New(),
	alpine.Name: alpine.New(),
	arch.Name:   arch.New(),
}

func knownDistroNames() []string {
	var ss []string
	for k := range knownDistros {
		ss = append(ss, k)
	}
	sort.Strings(ss)
	return ss
}

func getDistroByName(name string) (distro.Distro, error) {
	if name == "" {
		detected := detect.DistroID()
		if _, ok := knownDistros[detected]; ok {
			name = detected
		} else {
			logrus.Debugf("Unsupported distro %q", detected)
			name = none.Name
		}
	}
	if d, ok := knownDistros[name]; ok {
		return d, nil
	}
	return nil, fmt.Errorf("unknown distro %q (known distros: %v)", name, knownDistroNames())
}

func getDistro(cmd *cobra.Command) (distro.Distro, error) {
	name, err := cmd.Flags().GetString("distro")
	if err != nil {
		return nil, err
	}
	d, err := getDistroByName(name)
	if err != nil {
		return nil, err
	}
	info := d.Info()
	logrus.Debugf("Using distro driver %q", info.Name)
	if info.Experimental {
		logrus.Warnf("Distro driver %q is experimental", info.Name)
	}
	return d, nil
}
