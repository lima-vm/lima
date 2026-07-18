// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package kubernetesservice

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
)

type Protocol string

const (
	TCP Protocol = "tcp"
	UDP Protocol = "udp"
)

type Entry struct {
	Protocol Protocol
	IP       net.IP
	Port     uint16
}

type ServiceWatcher struct {
	rwMutex sync.RWMutex
	// key: namespace/name
	serviceSpecs map[string]*serviceSpec
}

func NewServiceWatcher() *ServiceWatcher {
	return &ServiceWatcher{serviceSpecs: make(map[string]*serviceSpec)}
}

func (s *ServiceWatcher) Start(ctx context.Context) {
	logrus.Info("Monitoring kubernetes services")
	go s.loopAttemptToStartKubectl(ctx)
}

func (s *ServiceWatcher) loopAttemptToStartKubectl(ctx context.Context) {
	const retryInterval = 10 * time.Second

	for i := 0; ; i++ {
		// The first iteration does not need to wait for retryInterval.
		if i > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryInterval):
			}
		}
		s.attemptToStartKubectl(ctx)
	}
}

func (s *ServiceWatcher) attemptToStartKubectl(ctx context.Context) {
	kubectl, err := exec.LookPath("kubectl")
	if err != nil {
		logrus.WithError(err).Debugf("kubectl not available; will retry")
		return
	}
	kubeconfig := chooseKubeconfig()
	// TODO: ensure that kubeconfig points to a local cluster
	if err := canGetServices(ctx, kubectl, kubeconfig); err != nil {
		logrus.WithError(err).Debugf("kubectl auth can-i ... failed; will retry")
		return
	}

	cmd := exec.CommandContext(ctx, kubectl,
		"get", "--all-namespaces", "service", "--watch", "--output-watch-events", "--output", "json")
	if kubeconfig != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)
	}
	if err := s.startAndStreamKubectl(cmd); err != nil {
		logrus.WithError(err).Warn("kubectl watch failed; will retry")
	}
}

func chooseKubeconfig() string {
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return kubeconfig
	}
	candidateKubeConfigs := []string{
		"/etc/rancher/k3s/k3s.yaml",
		"/root/.kube/config.localhost", // Created by template:k8s
		"/root/.kube/config",
	}
	for _, kc := range candidateKubeConfigs {
		if _, err := os.Stat(kc); !errors.Is(err, os.ErrNotExist) {
			return kc
		}
	}
	return ""
}

func canGetServices(ctx context.Context, kubectl, kubeconfig string) error {
	cmd := exec.CommandContext(ctx, kubectl, "auth", "can-i", "get", "service")
	if kubeconfig != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %v: %w; stdout=%q, stderr=%q", cmd.Args, err, stdout.String(), stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "yes" {
		return fmt.Errorf("failed to run %v: expected \"yes\", got %q", cmd.Args, stdout.String())
	}
	return nil
}

func (s *ServiceWatcher) startAndStreamKubectl(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to run %v: %w; stderr=%q", cmd.Args, err, stderr.String())
	}

	readErr := s.readKubectlStream(stdout)
	waitErr := cmd.Wait()
	if waitErr != nil {
		waitErr = fmt.Errorf("failed to run %v: %w; stderr=%q", cmd.Args, waitErr, stderr.String())
	}
	return errors.Join(readErr, waitErr)
}

// readKubectlStream reads kubectl JSON watch events from r and updates the internal
// services map. The stream is newline-delimited JSON objects representing "WatchEvent".
func (s *ServiceWatcher) readKubectlStream(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	// increase buffer in case of large JSON objects
	const maxBuf = 10 * units.MiB
	buf := make([]byte, 0, 64*units.KiB)
	scanner.Buffer(buf, maxBuf)

	for scanner.Scan() {
		line := scanner.Bytes()
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var ev struct {
			Type   eventType       `json:"type"`
			Object json.RawMessage `json:"object"`
		}
		if err := json.Unmarshal(line, &ev); err != nil {
			return fmt.Errorf("failed to unmarshal line %q: %w", string(line), err)
		}

		var svc service
		if err := json.Unmarshal(ev.Object, &svc); err != nil {
			return fmt.Errorf("failed to unmarshal service object: %w (line=%q)", err, line)
		}

		key := svc.Metadata.Namespace + "/" + svc.Metadata.Name
		s.rwMutex.Lock()
		switch ev.Type {
		case added, modified:
			s.serviceSpecs[key] = &svc.Spec
		case deleted:
			delete(s.serviceSpecs, key)
		default:
			// NOP
		}
		s.rwMutex.Unlock()
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan kubectl event stream: %w", err)
	}
	return nil
}

func (s *ServiceWatcher) GetPorts() []Entry {
	s.rwMutex.RLock()
	defer s.rwMutex.RUnlock()

	var entries []Entry
	for key, spec := range s.serviceSpecs {
		if spec.Type != serviceTypeNodePort &&
			spec.Type != serviceTypeLoadBalancer {
			continue
		}

		for _, portEntry := range spec.Ports {
			switch portEntry.Protocol {
			case protocolTCP, protocolUDP:
				// NOP
			default:
				logrus.Debugf("unsupported protocol %s for service %q, skipping",
					portEntry.Protocol, key)
				continue
			}

			var port int32
			switch spec.Type {
			case serviceTypeNodePort:
				port = portEntry.NodePort
			case serviceTypeLoadBalancer:
				port = portEntry.Port
			}

			entries = append(entries, Entry{
				Protocol: Protocol(strings.ToLower(string(portEntry.Protocol))),
				IP:       net.IPv4zero,
				Port:     uint16(port),
			})
		}
	}

	return entries
}
