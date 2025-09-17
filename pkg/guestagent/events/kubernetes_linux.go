// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

type event struct {
	UID         types.UID
	namespace   string
	name        string
	portMapping map[int32]corev1.Protocol
	deleted     bool
}

type KubeServiceWatcher struct {
	kubeConfigPaths []string
	kubeClient      kubernetes.Interface
	eventCh         chan event
	errorCh         chan error
}

func NewKubeServiceWatcher(cfgPaths []string) *KubeServiceWatcher {
	return &KubeServiceWatcher{
		kubeConfigPaths: cfgPaths,
		eventCh:         make(chan event),
		errorCh:         make(chan error),
	}
}

func (k *KubeServiceWatcher) createAndVerifyClient(ctx context.Context) (bool, error) {
	kubeClient, err := tryGetKubeClient(k.kubeConfigPaths)
	if err != nil {
		logrus.Tracef("failed to get kube client: %s", err)
		return false, nil
	}

	informerFactory := informers.NewSharedInformerFactory(kubeClient, time.Hour)
	serviceInformer := informerFactory.Core().V1().Services().Informer()

	_, err = serviceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			logrus.Tracef("Service Informer: Add func called with: %+v", obj)
			handleUpdate(nil, obj, k.eventCh)
		},
		DeleteFunc: func(obj any) {
			logrus.Tracef("Service Informer: Del func called with: %+v", obj)
			handleUpdate(obj, nil, k.eventCh)
		},
		UpdateFunc: func(oldObj, newObj any) {
			logrus.Tracef("Service Informer: Update func called with old object %+v and new Object: %+v", oldObj, newObj)
			handleUpdate(oldObj, newObj, k.eventCh)
		},
	})
	if err != nil {
		// this error can not be ignored and must be returned
		return false, fmt.Errorf("error setting eventHandler: %w", err)
	}
	err = serviceInformer.SetWatchErrorHandler(func(_ *cache.Reflector, err error) {
		k.errorCh <- fmt.Errorf("kubernetes: error watching service: %w", err)
	})
	if err != nil {
		// this error can not be ignored and must be returned
		return false, fmt.Errorf("error setting errorHandler: %w", err)
	}
	informerFactory.WaitForCacheSync(ctx.Done())
	informerFactory.Start(ctx.Done())
	k.kubeClient = kubeClient
	return true, nil
}

func (k *KubeServiceWatcher) initializeServices(ctx context.Context) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			services, err := k.kubeClient.CoreV1().Services(corev1.NamespaceAll).List(ctx, v1.ListOptions{})
			if err != nil {
				logrus.Errorf("Listing services failed: %s", err)
				switch {
				default:
					return err
				case isTimeout(err):
				case errors.Is(err, unix.ENETUNREACH):
				case errors.Is(err, unix.ECONNREFUSED):
				case isAPINotReady(err):
				}
				continue
			}

			// List the initial set of services asynchronously, so that we don't have to
			// worry about the channel blocking.
			go func() {
				for _, svc := range services.Items {
					handleUpdate(nil, svc, k.eventCh)
				}
			}()
			return nil

		case <-ctx.Done():
			return fmt.Errorf("context cancelled during initialization: %w", ctx.Err())
		}
	}
}

func (k *KubeServiceWatcher) MonitorServices(ctx context.Context, ch chan *api.Event) error {
	if err := tryGetClient(ctx, k.createAndVerifyClient); err != nil {
		return fmt.Errorf("failed getting kube client: %w", err)
	}

	if err := k.initializeServices(ctx); err != nil {
		return fmt.Errorf("failed initializing services: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancellation: %w", ctx.Err())
		case err := <-k.errorCh:
			logrus.Errorf("received an error from kube API: %s", err)
		case event := <-k.eventCh:
			logrus.Debugf("received an event from kube API: %+v", event)
			ipPorts := createIPPortFromPortMapping(event.portMapping)
			sendHostAgentEvent(event.deleted, ipPorts, ch)
		}
	}
}

func createIPPortFromPortMapping(portMapping map[int32]corev1.Protocol) (ipPorts []*api.IPPort) {
	for port, proto := range portMapping {
		ipPorts = append(ipPorts, &api.IPPort{
			Ip:       "0.0.0.0",
			Protocol: strings.ToLower(string(proto)),
			Port:     port,
		})
	}
	return ipPorts
}

func handleUpdate(oldObj, newObj any, eventCh chan<- event) {
	deleted := make(map[int32]corev1.Protocol)
	added := make(map[int32]corev1.Protocol)
	oldSvc, _ := oldObj.(*corev1.Service)
	newSvc, _ := newObj.(*corev1.Service)
	namespace := "<unknown>"
	name := "<unknown>"

	if oldSvc != nil {
		namespace = oldSvc.Namespace
		name = oldSvc.Name

		if oldSvc.Spec.Type == corev1.ServiceTypeNodePort {
			for _, port := range oldSvc.Spec.Ports {
				deleted[port.NodePort] = port.Protocol
			}
		}

		if oldSvc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			for _, port := range oldSvc.Spec.Ports {
				deleted[port.Port] = port.Protocol
			}
		}
	}

	if newSvc != nil {
		namespace = newSvc.Namespace
		name = newSvc.Name

		if newSvc.Spec.Type == corev1.ServiceTypeNodePort {
			for _, port := range newSvc.Spec.Ports {
				delete(deleted, port.NodePort)
				added[port.NodePort] = port.Protocol
			}
		}

		if newSvc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			for _, port := range newSvc.Spec.Ports {
				delete(deleted, port.Port)
				added[port.Port] = port.Protocol
			}
		}
	}

	if len(deleted) > 0 {
		sendEvents(deleted, oldSvc, true, eventCh)
	}

	if len(added) > 0 {
		sendEvents(added, newSvc, false, eventCh)
	}

	logrus.Debugf("kubernetes service update: %s/%s has -%d +%d service port",
		namespace, name, len(deleted), len(added))
}

func sendEvents(mapping map[int32]corev1.Protocol, svc *corev1.Service, deleted bool, eventCh chan<- event) {
	if svc != nil {
		eventCh <- event{
			UID:         svc.UID,
			namespace:   svc.Namespace,
			name:        svc.Name,
			portMapping: mapping,
			deleted:     deleted,
		}
	}
}

func tryGetKubeClient(candidateKubeConfigs []string) (kubernetes.Interface, error) {
	for _, kubeconfig := range candidateKubeConfigs {
		_, err := os.Stat(kubeconfig)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, fmt.Errorf("stat kubeconfig %s failed: %w", kubeconfig, err)
		}

		restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("build kubeconfig from %s failed: %w", kubeconfig, err)
		}
		u, err := url.Parse(restConfig.Host)
		if err != nil {
			return nil, fmt.Errorf("parse kubeconfig host %s failed: %w", restConfig.Host, err)
		}
		if u.Hostname() != "127.0.0.1" { // might need to support IPv6
			// ensures the kubeconfig points to local k8s
			continue
		}

		kubeClient, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return nil, err
		}

		return kubeClient, nil
	}

	return nil, errors.New("no valid kubeconfig found")
}

func isTimeout(err error) bool {
	type timeout interface {
		Timeout() bool
	}

	var timeoutError timeout

	return errors.As(err, &timeoutError) && timeoutError.Timeout()
}

// This is a k3s error that is received over
// the HTTP, Also, it is worth noting that this
// error is wrapped. This is why we are not testing
// against the real error object using errors.Is().
func isAPINotReady(err error) bool {
	return strings.Contains(err.Error(), "apiserver not ready") || strings.Contains(err.Error(), "starting")
}

func tryGetClient(ctx context.Context, tryConnect func(context.Context) (bool, error)) error {
	const retryInterval = 10 * time.Second
	const pollImmediately = true
	return wait.PollUntilContextCancel(ctx, retryInterval, pollImmediately, tryConnect)
}
