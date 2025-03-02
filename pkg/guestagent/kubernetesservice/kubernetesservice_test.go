/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubernetesservice

import (
	"context"
	"net"
	"testing"

	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/informers"
	clientSet "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func newFakeKubeClient() (clientSet.Interface, informers.SharedInformerFactory) {
	kubeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	return kubeClient, informerFactory
}

func TestGetPorts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serviceCreatedCh := make(chan struct{}, 1)
	kubeClient, informerFactory := newFakeKubeClient()
	serviceInformer := informerFactory.Core().V1().Services().Informer()
	_, err := serviceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(any) { serviceCreatedCh <- struct{}{} },
	})
	assert.NilError(t, err)
	informerFactory.Start(ctx.Done())
	serviceWatcher := NewServiceWatcher()
	serviceWatcher.setServiceInformer(serviceInformer)

	type testCase struct {
		name    string
		service corev1.Service
		want    []Entry
	}
	cases := []testCase{
		{
			name: "nodePort service",
			service: corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "nodeport"},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
					Ports: []corev1.ServicePort{
						{
							Name:       "http",
							Protocol:   corev1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromInt(80),
							NodePort:   8080,
						},
					},
				},
			},
			want: []Entry{{
				Protocol: TCP,
				IP:       net.ParseIP("0.0.0.0"),
				Port:     8080,
			}},
		},
		{
			name: "loadBalancer service",
			service: corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "loadbalancer"},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Name:       "http",
							Protocol:   corev1.ProtocolTCP,
							Port:       8081,
							TargetPort: intstr.FromInt(80),
						},
					},
				},
			},
			want: []Entry{{
				Protocol: TCP,
				IP:       net.ParseIP("0.0.0.0"),
				Port:     8081,
			}},
		},
		{
			name: "clusterIP service",
			service: corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "clusterip"},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
					Ports: []corev1.ServicePort{
						{
							Name:       "http",
							Protocol:   corev1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromInt(80),
						},
					},
				},
			},
			want: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := kubeClient.CoreV1().Services("default").Create(ctx, &c.service, metav1.CreateOptions{})
			assert.NilError(t, err, "failed to create service [%s]", c.service.Name)

			<-serviceCreatedCh

			got := serviceWatcher.GetPorts()
			assert.DeepEqual(t, got, c.want)
			err = kubeClient.CoreV1().Services("default").Delete(ctx, c.service.Name, metav1.DeleteOptions{})
			assert.NilError(t, err, "failed to delete service [%s]", c.service.Name)
		})
	}
}
