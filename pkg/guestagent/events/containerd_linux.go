// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/errdefs"
	ctrns "github.com/containerd/containerd/namespaces"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

const (
	stateKey             = "nerdctl/state-dir"
	portsKey             = "nerdctl/ports"
	namespaceKey         = "nerdctl/namespace"
	defaultSocketTimeout = 5 * time.Second
)

type ipPortMap map[string][]*api.IPPort

type ContainerdEventMonitor struct {
	socketPaths []string
}

func NewContainerdEventMonitor(socketPaths []string) *ContainerdEventMonitor {
	return &ContainerdEventMonitor{
		socketPaths: socketPaths,
	}
}

func (c *ContainerdEventMonitor) MonitorPorts(ctx context.Context, ch chan *api.Event) {
	const defaultRetryDelay = 2
	retryDelay := 0
	var wg sync.WaitGroup
	for _, socket := range c.socketPaths {
		wg.Add(1)
		go func(socket string) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(retryDelay) * time.Second):
					logrus.Debugf("attempting to connect to containerd socket %s after: %d", socket, retryDelay)
					retryDelay = defaultRetryDelay
				}
				info, err := os.Stat(socket)
				if err != nil {
					if os.IsNotExist(err) {
						logrus.Warnf("containerd socket %s does not exist", socket)
					} else {
						logrus.Errorf("failed to stat containerd socket %s: %v", socket, err)
					}
					retryDelay = defaultRetryDelay
					continue
				}
				if info.IsDir() {
					logrus.Errorf("containerd socket path %s is a directory", socket)
					retryDelay = 15
					continue
				}
				client, err := containerd.New(socket, containerd.WithDefaultNamespace(ctrns.Default))
				if err != nil {
					logrus.Warnf("failed to create client for socket %s: %v", socket, err)
					continue
				}
				logrus.Debugf("created containerd client for socket %s", socket)
				clientCtx, cancel := context.WithTimeout(ctx, defaultSocketTimeout)
				serving, serveErr := client.IsServing(clientCtx)
				cancel()
				if serveErr != nil || !serving {
					logrus.Warnf("containerd daemon not serving on socket %s: %v. Retrying in 5s...", socket, serveErr)
					client.Close()
					retryDelay = defaultRetryDelay
					continue
				}
				logrus.Infof("successfully connected to containerd on socket %s", socket)
				if err := runMonitorClient(ctx, client, ch); err != nil {
					logrus.Errorf("containerd port monitoring for socket: %s failed: %s", socket, err)
				}
				client.Close()
			}
		}(socket)
	}
	wg.Wait()
}

func runMonitorClient(ctx context.Context, client *containerd.Client, ch chan *api.Event) error {
	runningContainers := make(ipPortMap)
	subscribeFilters := []string{
		`topic=="/tasks/start"`,
		`topic=="/containers/update"`,
		`topic=="/tasks/exit"`,
	}
	msgCh, errCh := client.Subscribe(ctx, subscribeFilters...)

	if err := initializeRunningContainers(ctx, client, ch, runningContainers); err != nil {
		return fmt.Errorf("failed to initialize existing containers published ports: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancellation: %w", ctx.Err())

		case err := <-errCh:
			return fmt.Errorf("receiving container event failed: %w", err)

		case envelope := <-msgCh:
			logrus.Debugf("received an event: %+v", envelope.Topic)
			switch envelope.Topic {
			case "/tasks/start":
				taskStart := &events.TaskStart{}
				err := proto.Unmarshal(envelope.Event.GetValue(), taskStart)
				if err != nil {
					logrus.Errorf("failed to unmarshal TaskStart event: %v", err)
					continue
				}

				ipPorts, err := createIPPort(ctx, client, envelope.Namespace, taskStart.ContainerID)
				if err != nil {
					logrus.Errorf("creating IPPorts for start task ContainerID=%s failed: %s", taskStart.ContainerID, err)
					continue
				}

				logrus.Debugf("received the following TaskStart: ContainerID=%s ipPorts=%+v", taskStart.ContainerID, ipPorts)

				if len(ipPorts) != 0 {
					sendHostAgentEvent(false, ipPorts, ch)
					runningContainers[taskStart.ContainerID] = ipPorts
				}

			case "/containers/update":
				cuEvent := &events.ContainerUpdate{}
				err := proto.Unmarshal(envelope.Event.GetValue(), cuEvent)
				if err != nil {
					logrus.Errorf("failed to unmarshal container update event: %v", err)
					continue
				}

				ipPorts, err := createIPPort(ctx, client, envelope.Namespace, cuEvent.ID)
				if err != nil {
					logrus.Errorf("creating IPPorts, for the following exit task: %v failed: %s", cuEvent, err)
					continue
				}

				logrus.Debugf("received the following updateTask: %v for: %v", cuEvent, ipPorts)

				if existingipPorts, ok := runningContainers[cuEvent.ID]; ok {
					if !ipPortsEqual(ipPorts, existingipPorts) {
						// first remove the existing entry
						sendHostAgentEvent(true, existingipPorts, ch)
						// then update with the new entry
						sendHostAgentEvent(false, ipPorts, ch)
						runningContainers[cuEvent.ID] = ipPorts
					}
				}
			case "/tasks/exit":
				exitTask := &events.TaskExit{}
				err := proto.Unmarshal(envelope.Event.GetValue(), exitTask)
				if err != nil {
					logrus.Errorf("failed to unmarshal container's exit task: %v", err)
					continue
				}

				container, err := client.LoadContainer(ctx, exitTask.ContainerID)
				if err != nil {
					if errdefs.IsNotFound(err) {
						logrus.Debugf("container: %s in namespace: %s not found, deleting port mapping", exitTask.ContainerID, envelope.Namespace)
						deleteRunningContainer(exitTask.ContainerID, ch, runningContainers)
						continue
					}
					logrus.Errorf("failed to get the container %s from namespace %s: %s", exitTask.ContainerID, envelope.Namespace, err)
					continue
				}

				tsk, err := container.Task(ctx, nil)
				if err != nil {
					if errdefs.IsNotFound(err) {
						logrus.Debugf("task for container %s in namespace %s not found, deleting port mapping", exitTask.ContainerID, envelope.Namespace)
						deleteRunningContainer(exitTask.ContainerID, ch, runningContainers)
						continue
					}
					logrus.Errorf("failed to get the task for container %s: %s", exitTask.ContainerID, err)
					continue
				}
				status, err := tsk.Status(ctx)
				if err != nil {
					logrus.Errorf("failed to get the task status for container %s: %s", exitTask.ContainerID, err)
					continue
				}

				if status.Status == containerd.Running {
					logrus.Debugf("container %s is still running, but received exit event with status %d", exitTask.ContainerID, exitTask.ExitStatus)
					continue
				}

				deleteRunningContainer(exitTask.ContainerID, ch, runningContainers)
			}
		}
	}
}

func deleteRunningContainer(containerID string, ch chan *api.Event, runningContainers ipPortMap) {
	if ipPorts, ok := runningContainers[containerID]; ok {
		delete(runningContainers, containerID)
		logrus.Debugf("deleted container %s from running containers", containerID)
		sendHostAgentEvent(true, ipPorts, ch)
	} else {
		logrus.Debugf("container %s not found in running containers", containerID)
	}
}

func initializeRunningContainers(ctx context.Context, client *containerd.Client, ch chan *api.Event, runningContainers ipPortMap) error {
	containers, err := client.Containers(ctx)
	if err != nil {
		return err
	}

	for _, container := range containers {
		task, err := container.Task(ctx, nil)
		if err != nil || task == nil {
			logrus.Errorf("failed getting container %s task: %s", container.ID(), err)
			continue
		}

		status, err := task.Status(ctx)
		if err != nil || status.Status != containerd.Running {
			logrus.Errorf("failed getting container %s task status: %s", container.ID(), err)
			continue
		}

		labels, err := container.Labels(ctx)
		if err != nil {
			logrus.Errorf("failed getting container %s labels: %s", container.ID(), err)
			continue
		}

		namespace, ok := labels[namespaceKey]
		if !ok {
			logrus.Errorf("container %s does not have a namespace label", container.ID())
			continue
		}
		ipPorts, err := createIPPort(ctx, client, namespace, container.ID())
		if err != nil {
			logrus.Errorf("creating IPPorts, while initializing containers the following: %v failed: %s", container.ID(), err)
			continue
		}

		sendHostAgentEvent(false, ipPorts, ch)
		runningContainers[container.ID()] = ipPorts
	}

	return nil
}

func createIPPort(ctx context.Context, client *containerd.Client, namespace, containerID string) ([]*api.IPPort, error) {
	container, err := client.ContainerService().Get(
		ctrns.WithNamespace(ctx, namespace), containerID)
	if err != nil {
		return nil, err
	}

	var ipPorts []*api.IPPort

	// For backward compatibility, we first check if the container has the nerdctl/ports label.
	// If it does, we parse it and return the IPPorts.
	containerPorts, ok := container.Labels[portsKey]
	if ok {
		ipPorts, err = extractIPPortsFromLabel(containerPorts)
		if err != nil {
			return nil, fmt.Errorf("extracting IPPorts from container %s ports label failed: %w", containerID, err)
		}
		return ipPorts, nil
	}
	// If the label is not present, we check the network config in the following path:
	// <DATAROOT>/<ADDRHASH>/containers/<NAMESPACE>/<CID>/network-config.json
	stateDir, ok := container.Labels[stateKey]
	if !ok {
		return nil, fmt.Errorf("container %s does not have a state directory label", containerID)
	}
	content, err := os.ReadFile(fmt.Sprintf("%s/network-config.json", stateDir))
	if err != nil {
		return nil, fmt.Errorf("failed reading network-config.json in dir %s for container %s: %w", stateDir, containerID, err)
	}
	return extractIPPortsFromNetworkConfig(content)
}

func extractIPPortsFromLabel(jsonPorts string) ([]*api.IPPort, error) {
	var ports []Port
	err := json.Unmarshal([]byte(jsonPorts), &ports)
	if err != nil {
		return nil, err
	}

	var ipPorts []*api.IPPort
	for _, port := range ports {
		ipPorts = append(ipPorts, &api.IPPort{
			Protocol: strings.ToLower(port.Protocol),
			Ip:       port.HostIP,
			Port:     int32(port.HostPort),
		})
	}

	return ipPorts, nil
}

func extractIPPortsFromNetworkConfig(jsonStr []byte) ([]*api.IPPort, error) {
	var cfg NetworkConfig
	if err := json.Unmarshal(jsonStr, &cfg); err != nil {
		return nil, err
	}

	var ipPorts []*api.IPPort
	for _, port := range cfg.PortMappings {
		ipPorts = append(ipPorts, &api.IPPort{
			Protocol: strings.ToLower(port.Protocol),
			Ip:       port.HostIP,
			Port:     int32(port.HostPort),
		})
	}
	return ipPorts, nil
}

func ipPortsEqual(a, b []*api.IPPort) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Protocol != b[i].Protocol || a[i].Ip != b[i].Ip || a[i].Port != b[i].Port {
			return false
		}
	}
	return true
}

// Port is representing nerdctl/ports entry in the
// event envelope's labels.
type Port struct {
	HostPort      int    `json:"HostPort"`
	ContainerPort int    `json:"ContainerPort"`
	Protocol      string `json:"Protocol"`
	HostIP        string `json:"HostIP"`
}

// NetworkConfig is representing the network config
// of a container that is found in the following Path:
// <DATAROOT>/<ADDRHASH>/containers/<NAMESPACE>/<CID>/network-config.json.
type NetworkConfig struct {
	PortMappings []Port `json:"portMappings"`
}
