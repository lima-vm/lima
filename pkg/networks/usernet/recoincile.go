// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usernet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/lima-vm/lima/pkg/executil"
	"github.com/lima-vm/lima/pkg/lockutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/sirupsen/logrus"
)

// Start starts a instance a usernet network with the given name.
// The name parameter must point to a valid network configuration name under <LIMA_HOME>/_config/networks.yaml with `mode: user-v2`.
func Start(ctx context.Context, name string) error {
	logrus.Debugf("Make sure usernet network is started")
	networksDir, err := dirnames.LimaNetworksDir()
	if err != nil {
		return err
	}
	// usernet files contents are stored under {LIMA_HOME}/_networks/user-v2/<pid, fdsock, endpointsock, logs>
	usernetDir := path.Join(networksDir, name)
	if err := os.MkdirAll(usernetDir, 0o755); err != nil {
		return err
	}

	pidFile, err := PIDFile(name)
	if err != nil {
		return err
	}
	pid, _ := store.ReadPIDFile(pidFile)
	if pid == 0 {
		qemuSock, err := Sock(name, QEMUSock)
		if err != nil {
			return err
		}

		fdSock, err := Sock(name, FDSock)
		if err != nil {
			return err
		}

		endpointSock, err := Sock(name, EndpointSock)
		if err != nil {
			return err
		}

		subnet, err := SubnetCIDR(name)
		if err != nil {
			return err
		}

		leases, err := readLeases(name)
		if err != nil {
			return err
		}

		err = lockutil.WithDirLock(usernetDir, func() error {
			self, err := os.Executable()
			if err != nil {
				return err
			}
			leasesString := mapToCliString(leases)
			args := []string{
				"usernet", "-p", pidFile,
				"-e", endpointSock,
				"--listen-qemu", qemuSock,
				"--listen", fdSock,
				"--subnet", subnet.String(),
			}
			if leasesString != "" {
				args = append(args, "--leases", leasesString)
			}
			cmd := exec.CommandContext(ctx, self, args...)
			cmd.SysProcAttr = executil.BackgroundSysProcAttr

			stdoutPath := filepath.Join(usernetDir, fmt.Sprintf("%s.%s.%s.log", "usernet", name, "stdout"))
			stderrPath := filepath.Join(usernetDir, fmt.Sprintf("%s.%s.%s.log", "usernet", name, "stderr"))
			if err := os.RemoveAll(stdoutPath); err != nil {
				return err
			}
			if err := os.RemoveAll(stderrPath); err != nil {
				return err
			}

			cmd.Stdout, err = os.Create(stdoutPath)
			if err != nil {
				return err
			}
			cmd.Stderr, err = os.Create(stderrPath)
			if err != nil {
				return err
			}

			logrus.Debugf("Starting usernet network: %v", cmd.Args)
			if err := cmd.Start(); err != nil {
				return fmt.Errorf("failed to run %v: %w (Hint: check %s/usernet.*.log)", cmd.Args, err, usernetDir)
			}
			return nil
		})
		if err != nil {
			return err
		}
		for {
			if _, err := os.Stat(fdSock); !errors.Is(err, os.ErrNotExist) {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}

// Stop stops running instance a usernet network with the given name.
// The name parameter must point to a valid network configuration name under <LIMA_HOME>/_config/networks.yaml with `mode: user-v2`.
func Stop(ctx context.Context, name string) error {
	logrus.Debugf("Make sure usernet network is stopped")
	pidFile, err := PIDFile(name)
	if err != nil {
		return err
	}
	pid, _ := store.ReadPIDFile(pidFile)

	if pid != 0 {
		logrus.Debugf("Stopping usernet daemon")

		err = writeLeases(ctx, name)
		if err != nil {
			return err
		}

		var stdout, stderr bytes.Buffer
		cmd := exec.Command("/usr/bin/pkill", "-F", pidFile)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		logrus.Debugf("Running: %v", cmd.Args)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run %v: stdout=%q, stderr=%q: %w",
				cmd.Args, stdout.String(), stderr.String(), err)
		}
	}

	// wait for daemons to terminate (up to 5s) before stopping, otherwise the sockets may not get deleted which
	// will cause subsequent start commands to fail.
	startWaiting := time.Now()
	for {
		if pid, _ := store.ReadPIDFile(pidFile); pid == 0 {
			break
		}
		if time.Since(startWaiting) > 5*time.Second {
			logrus.Infof("usernet network still running after 5 seconds")
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

func mapToCliString(m map[string]string) string {
	var strArr []string
	for key, value := range m {
		strArr = append(strArr, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(strArr, ",")
}

func readLeases(name string) (map[string]string, error) {
	leasesFile, err := Leases(name)
	if err != nil {
		return nil, err
	}
	var leases map[string]string
	if _, err := os.Stat(leasesFile); errors.Is(err, os.ErrNotExist) {
		return leases, nil
	}
	file, err := os.Open(leasesFile)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&leases)
	return leases, err
}

func writeLeases(ctx context.Context, nwName string) error {
	client := NewClientByName(nwName)
	leases, err := client.Leases(ctx)
	if err != nil {
		return err
	}
	leasesFile, err := Leases(nwName)
	if err != nil {
		return err
	}
	file, err := os.Create(leasesFile)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	err = encoder.Encode(leases)
	if err != nil {
		return err
	}
	return nil
}
