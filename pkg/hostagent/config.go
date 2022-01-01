package hostagent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-multierror"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/sirupsen/logrus"
)

func (a *HostAgent) copyConfigFiles(ctx context.Context) error {
	var (
		mErr error
	)
	for _, f := range a.y.ConfigFiles {
		logrus.Infof("Copying %q (guest) to %q (host)", f.GuestConfig, f.HostConfig)
		err := a.copyConfigFile(ctx, f)
		if err != nil {
			mErr = multierror.Append(mErr, err)
			continue
		}
	}
	return mErr
}

func (a *HostAgent) copyConfigFile(ctx context.Context, c limayaml.ConfigFile) error {
	script := "#!/bin/sh\n"
	if c.Mode == limayaml.ProvisionModeSystem {
		script += "sudo "
	}
	script += "cat" + " " + c.GuestConfig
	description := fmt.Sprintf("%s -> %s", c.GuestConfig, c.HostConfig)
	logrus.Debugf("executing script %q", description)
	stdout, stderr, err := ssh.ExecuteScript("127.0.0.1", a.sshLocalPort, a.sshConfig, script, description)
	if err != nil {
		return fmt.Errorf("stderr=%q: %w", stderr, err)
	}
	conf := filepath.Dir(c.HostConfig)
	if err := os.MkdirAll(conf, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(c.HostConfig, []byte(stdout), 0644); err != nil {
		return err
	}
	return nil
}
