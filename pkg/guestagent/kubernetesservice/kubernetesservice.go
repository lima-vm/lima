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

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type Protocol string

const (
	// UDP/SCTP when lima port forwarding works on those protocols.

	TCP Protocol = "tcp"
)

type Entry struct {
	Protocol Protocol
	IP       net.IP
	Port     uint16
}

type ServiceWatcher struct {
	rwMutex sync.RWMutex
	// key: namespace/name
	services map[string]*corev1.Service
}

func NewServiceWatcher() *ServiceWatcher {
	return &ServiceWatcher{services: make(map[string]*corev1.Service)}
}

func (s *ServiceWatcher) Start(ctx context.Context) {
	logrus.Info("Monitoring kubernetes services")
	const retryInterval = 10 * time.Second

	go func() {
		for i := 0; ; i++ {
			if i > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(retryInterval):
				}
			}

			kubectl, err := exec.LookPath("kubectl")
			if err != nil {
				logrus.WithError(err).Debugf("kubectl not available; will retry")
				continue
			}
			kubeconfig := chooseKubeconfig()
			// TODO: ensure that kubeconfig points to a local cluster
			if err := canGetServices(ctx, kubectl, kubeconfig); err != nil {
				logrus.WithError(err).Debugf("kubectl auth can-i ... failed; will retry")
				continue
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
	}()
}

func chooseKubeconfig() string {
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return kubeconfig
	}
	candidateKubeConfigs := []string{
		"/etc/rancher/k3s/k3s.yaml",
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
	const maxBuf = 10 * 1024 * 1024
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxBuf)

	for scanner.Scan() {
		line := scanner.Bytes()
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var ev struct {
			Type   watch.EventType `json:"type"`
			Object json.RawMessage `json:"object"`
		}
		if err := json.Unmarshal(line, &ev); err != nil {
			return fmt.Errorf("failed to unmarshal line %q: %w", string(line), err)
		}

		var svc corev1.Service
		if err := json.Unmarshal(ev.Object, &svc); err != nil {
			return fmt.Errorf("failed to unmarshal service object: %w (line=%q)", err, line)
		}

		key := svc.Namespace + "/" + svc.Name
		s.rwMutex.Lock()
		switch ev.Type {
		case watch.Added, watch.Modified:
			s.services[key] = &svc
		case watch.Deleted:
			delete(s.services, key)
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
	for _, service := range s.services {
		if service.Spec.Type != corev1.ServiceTypeNodePort &&
			service.Spec.Type != corev1.ServiceTypeLoadBalancer {
			continue
		}

		for _, portEntry := range service.Spec.Ports {
			if portEntry.Protocol != corev1.ProtocolTCP {
				// currently only TCP port can be forwarded
				continue
			}

			var port int32
			switch service.Spec.Type {
			case corev1.ServiceTypeNodePort:
				port = portEntry.NodePort
			case corev1.ServiceTypeLoadBalancer:
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
