// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usernet

import (
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

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/executil"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/lockutil"
	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/store"
)

// Start starts a instance a usernet network with the given name.
// The name parameter must point to a valid network configuration name under <LIMA_HOME>/_config/networks.yaml with `mode: user-v2`.
func Start(ctx context.Context, name string) error {
	logrus.Debugf("Make sure usernet network is started")
	if external, err := isExternal(name); err != nil {
		return err
	} else if external {
		// an external daemon must already be running, Lima never starts its own instead
		return CheckExternal(ctx, name)
	}
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
	if external, err := isExternal(name); err != nil {
		return err
	} else if external {
		// the PID file of an external network is not written by Lima, so the process must not be signaled
		logrus.Debugf("Not stopping external usernet network %#q", name)
		return nil
	}
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

		if err := osutil.SysKill(pid, osutil.SigInt); err != nil {
			logrus.Error(err)
			return fmt.Errorf("failed to kill process with pid %d: %w", pid, err)
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
			logrus.Infof("usernet network still running after 5 seconds. Attempting to forcibly kill")
			if err := osutil.SysKill(pid, osutil.SigKill); err != nil {
				logrus.Error(err)
			}
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

func isExternal(name string) (bool, error) {
	cfg, err := networks.LoadConfig()
	if err != nil {
		return false, err
	}
	return cfg.External(name)
}

// externalProbeTimeout is the time CheckExternal waits for the API of the daemon to respond.
const externalProbeTimeout = 5 * time.Second

// CheckExternal returns an error if the daemon of the given external usernet network is not running.
//
// Only the endpoint socket is checked, because it is the only part of the daemon that is used regardless of
// the VM type. The data socket (e.g., the fd socket for vz, the qemu socket for qemu) is dialed by the
// driver when the VM is created, and that dial fails the start when the socket is missing or stale. Requiring
// the data sockets here would reject daemons that only serve the transport their VM type needs.
func CheckExternal(ctx context.Context, name string) error {
	sock, err := Sock(name, EndpointSock)
	if err != nil {
		return err
	}
	fi, err := os.Stat(sock)
	if err != nil {
		return fmt.Errorf("external usernet network %#q is not running (Lima never starts the daemon of an external network): %w", name, err)
	}
	if fi.Mode().Type() != os.ModeSocket {
		return fmt.Errorf("external usernet network %#q: %#q is not a socket", name, sock)
	}
	// The socket is left behind when the daemon is killed, so its existence is not enough.
	// The leases API is probed because Lima needs it anyway to resolve the IP address of the guest.
	client := NewClientByName(name)
	if client == nil {
		return fmt.Errorf("external usernet network %#q: failed to create the API client", name)
	}
	ctx, cancel := context.WithTimeout(ctx, externalProbeTimeout)
	defer cancel()
	if _, err := client.Leases(ctx); err != nil {
		return fmt.Errorf("external usernet network %#q is not responding on %#q: %w", name, sock, err)
	}
	logrus.Debugf("External usernet network %#q is up", name)
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
