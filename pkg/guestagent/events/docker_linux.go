// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

type DockerEventMonitor struct {
	dockerSocketPaths []string
}

func NewDockerEventMonitor(dockerSocketPaths []string) *DockerEventMonitor {
	return &DockerEventMonitor{
		dockerSocketPaths: dockerSocketPaths,
	}
}

func (d *DockerEventMonitor) MonitorPorts(ctx context.Context, ch chan *api.Event) {
	const defaultRetryDelay = 2
	retryDelay := 0
	var wg sync.WaitGroup
	for _, socket := range d.dockerSocketPaths {
		wg.Add(1)
		go func(socket string) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(retryDelay) * time.Second):
					logrus.Debugf("attempting to connect to docker socket %s after: %d", socket, retryDelay)
					retryDelay = defaultRetryDelay
				}

				info, err := os.Stat(socket)
				if err != nil {
					if os.IsNotExist(err) {
						logrus.Warnf("Docker socket %s does not exist: %s", socket, err)
					} else {
						logrus.Errorf("failed to stat docker socket: %s: %s", socket, err)
					}
					retryDelay = defaultRetryDelay
					continue
				}
				if info.IsDir() {
					logrus.Errorf("docker socket path %s is a directory", socket)
					retryDelay = 15
					continue
				}

				var socketURL string
				if !strings.HasPrefix(socket, "unix://") {
					if strings.HasPrefix(socket, "/") {
						socketURL = "unix://" + strings.Trim(socket, "/")
					} else {
						socketURL = "unix://" + socket
					}
				}

				client, err := client.NewClientWithOpts(client.WithHost(socketURL), client.WithAPIVersionNegotiation())
				if err != nil {
					logrus.Errorf("failed to create a docker client %s", err)
					continue
				}
				clientCtx, cancel := context.WithTimeout(ctx, defaultSocketTimeout)
				_, err = client.Ping(clientCtx)
				cancel()
				if err != nil {
					logrus.Warnf("docker daemon not serving on socket %s: %v. Retrying in 5s...", socket, err)
					client.Close()
					retryDelay = defaultRetryDelay
					continue
				}
				logrus.Infof("successfully connected to docker on socket %s", socket)
				if err := d.runMonitorClient(ctx, client, ch); err != nil {
					logrus.Errorf("docker port monitoring for socket: %s failed: %s", socket, err)
				}
				client.Close()
			}
		}(socket)
	}
	wg.Wait()
}

func (d *DockerEventMonitor) runMonitorClient(ctx context.Context, cli *client.Client, ch chan *api.Event) error {
	runningContainers := make(ipPortMap)
	defer cli.Close()

	if err := d.initializeRunningContainers(ctx, cli, ch, runningContainers); err != nil {
		logrus.Errorf("failed to initialize existing docker container published ports: %s", err)
	}

	msgCh, errCh := cli.Events(ctx, events.ListOptions{
		Filters: filters.NewArgs(
			filters.Arg("type", string(types.ContainerObject)),
			filters.Arg("event", string(events.ActionStart)),
			filters.Arg("event", string(events.ActionStop)),
			filters.Arg("event", string(events.ActionDie))),
	})

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancellation: %w", ctx.Err())

		case event := <-msgCh:
			container, err := cli.ContainerInspect(ctx, event.Actor.ID)
			if err != nil {
				logrus.Errorf("inspecting container [%v] failed: %v", event.Actor.ID, err)
				continue
			}
			portMap := container.NetworkSettings.NetworkSettingsBase.Ports
			logrus.Debugf("received an event: {Status: %+v ContainerID: %+v Ports: %+v}",
				event.Action,
				event.Actor.ID,
				portMap)

			switch event.Action {
			case events.ActionStart:
				if len(portMap) != 0 {
					validatePortMapping(portMap)
					ipPorts, err := convertToIPPort(portMap)
					if err != nil {
						logrus.Errorf("converting docker's portMapping: %+v to api.IPPort: %v failed: %s", portMap, ipPorts, err)
						continue
					}
					logrus.Infof("successfully converted PortMapping:%+v to IPPorts: %+v", portMap, ipPorts)
					runningContainers[event.Actor.ID] = ipPorts
					sendHostAgentEvent(false, ipPorts, ch)
				}
			case events.ActionStop, events.ActionDie:
				ipPorts, ok := runningContainers[event.Actor.ID]
				if ok {
					delete(runningContainers, event.Actor.ID)
				}
				if ok {
					sendHostAgentEvent(true, ipPorts, ch)
				}
			}
		case err := <-errCh:
			return fmt.Errorf("receiving container event failed: %w", err)
		}
	}
}

func (d *DockerEventMonitor) initializeRunningContainers(ctx context.Context, cli *client.Client, ch chan *api.Event, runningContainers ipPortMap) error {
	containers, err := cli.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("status", "running")),
	})
	if err != nil {
		return err
	}

	for _, container := range containers {
		if len(container.Ports) == 0 {
			continue
		}
		var ipPorts []*api.IPPort
		for _, port := range container.Ports {
			if port.IP == "" || port.PublicPort == 0 {
				continue
			}

			ipPorts = append(ipPorts, &api.IPPort{
				Protocol: strings.ToLower(port.Type),
				Ip:       port.IP,
				Port:     int32(port.PublicPort),
			})
		}
		sendHostAgentEvent(false, ipPorts, ch)
		runningContainers[container.ID] = ipPorts
	}
	return nil
}

func convertToIPPort(portMap nat.PortMap) ([]*api.IPPort, error) {
	var ipPorts []*api.IPPort
	for key, portBindings := range portMap {
		for _, portBinding := range portBindings {
			hostPort, err := strconv.ParseInt(portBinding.HostPort, 10, 32)
			if err != nil {
				return ipPorts, err
			}
			if portBinding.HostIP == "" || hostPort == 0 {
				continue
			}

			logrus.Debugf("converted the following PortMapping to IPPort, containerPort:%v HostPort:%v IP:%v Protocol:%v",
				key.Port(), portBinding.HostPort, portBinding.HostIP, key.Proto())

			ipPorts = append(ipPorts, &api.IPPort{
				Protocol: strings.ToLower(key.Proto()),
				Ip:       portBinding.HostIP,
				Port:     int32(hostPort),
			})
		}
	}
	return ipPorts, nil
}

// Removes entries in port mapping that do not hold any values
// for IP and Port e.g 9000/tcp:[].
func validatePortMapping(portMap nat.PortMap) {
	for k, v := range portMap {
		if len(v) == 0 {
			logrus.Debugf("removing entry: %v from the portmappings: %v", k, portMap)
			delete(portMap, k)
		}
	}
}
