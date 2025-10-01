// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package kubernetesservice

import (
	"net"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestGetPorts(t *testing.T) {
	serviceWatcher := NewServiceWatcher()

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
						{
							Name:       "dns",
							Protocol:   corev1.ProtocolUDP,
							Port:       53,
							TargetPort: intstr.FromInt(53),
							NodePort:   5353,
						},
					},
				},
			},
			want: []Entry{{
				Protocol: TCP,
				IP:       net.IPv4zero,
				Port:     8080,
			}, {
				Protocol: UDP,
				IP:       net.IPv4zero,
				Port:     5353,
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
				IP:       net.IPv4zero,
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
			const ns = "default"
			service := c.service
			service.Namespace = ns
			key := ns + "/" + c.service.Name

			serviceWatcher.rwMutex.Lock()
			serviceWatcher.serviceSpecs[key] = &service.Spec
			serviceWatcher.rwMutex.Unlock()

			got := serviceWatcher.GetPorts()
			assert.DeepEqual(t, got, c.want)

			serviceWatcher.rwMutex.Lock()
			delete(serviceWatcher.serviceSpecs, key)
			serviceWatcher.rwMutex.Unlock()
		})
	}
}

func TestReadKubectlStream(t *testing.T) {
	stream := `{"type":"ADDED","object":{"apiVersion":"v1","kind":"Service","metadata":{"creationTimestamp":"2025-09-30T13:39:19Z","labels":{"component":"apiserver","provider":"kubernetes"},"managedFields":[{"apiVersion":"v1","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:labels":{".":{},"f:component":{},"f:provider":{}}},"f:spec":{"f:clusterIP":{},"f:internalTrafficPolicy":{},"f:ipFamilyPolicy":{},"f:ports":{".":{},"k:{\"port\":443,\"protocol\":\"TCP\"}":{".":{},"f:name":{},"f:port":{},"f:protocol":{},"f:targetPort":{}}},"f:sessionAffinity":{},"f:type":{}}},"manager":"kube-apiserver","operation":"Update","time":"2025-09-30T13:39:19Z"}],"name":"kubernetes","namespace":"default","resourceVersion":"201","uid":"de36e2fb-3883-47f2-860a-c28dfd896bac"},"spec":{"clusterIP":"10.96.0.1","clusterIPs":["10.96.0.1"],"internalTrafficPolicy":"Cluster","ipFamilies":["IPv4"],"ipFamilyPolicy":"SingleStack","ports":[{"name":"https","port":443,"protocol":"TCP","targetPort":6443}],"sessionAffinity":"None","type":"ClusterIP"},"status":{"loadBalancer":{}}}}
{"type":"ADDED","object":{"apiVersion":"v1","kind":"Service","metadata":{"annotations":{"prometheus.io/port":"9153","prometheus.io/scrape":"true"},"creationTimestamp":"2025-09-30T13:39:19Z","labels":{"k8s-app":"kube-dns","kubernetes.io/cluster-service":"true","kubernetes.io/name":"CoreDNS"},"managedFields":[{"apiVersion":"v1","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:annotations":{".":{},"f:prometheus.io/port":{},"f:prometheus.io/scrape":{}},"f:labels":{".":{},"f:k8s-app":{},"f:kubernetes.io/cluster-service":{},"f:kubernetes.io/name":{}}},"f:spec":{"f:clusterIP":{},"f:internalTrafficPolicy":{},"f:ports":{".":{},"k:{\"port\":53,\"protocol\":\"TCP\"}":{".":{},"f:name":{},"f:port":{},"f:protocol":{},"f:targetPort":{}},"k:{\"port\":53,\"protocol\":\"UDP\"}":{".":{},"f:name":{},"f:port":{},"f:protocol":{},"f:targetPort":{}},"k:{\"port\":9153,\"protocol\":\"TCP\"}":{".":{},"f:name":{},"f:port":{},"f:protocol":{},"f:targetPort":{}}},"f:selector":{},"f:sessionAffinity":{},"f:type":{}}},"manager":"kubeadm","operation":"Update","time":"2025-09-30T13:39:19Z"}],"name":"kube-dns","namespace":"kube-system","resourceVersion":"240","uid":"73817952-5f95-4a15-a6f9-64ba220a6933"},"spec":{"clusterIP":"10.96.0.10","clusterIPs":["10.96.0.10"],"internalTrafficPolicy":"Cluster","ipFamilies":["IPv4"],"ipFamilyPolicy":"SingleStack","ports":[{"name":"dns","port":53,"protocol":"UDP","targetPort":53},{"name":"dns-tcp","port":53,"protocol":"TCP","targetPort":53},{"name":"metrics","port":9153,"protocol":"TCP","targetPort":9153}],"selector":{"k8s-app":"kube-dns"},"sessionAffinity":"None","type":"ClusterIP"},"status":{"loadBalancer":{}}}}
{"type":"ADDED","object":{"apiVersion":"v1","kind":"Service","metadata":{"creationTimestamp":"2025-10-01T09:20:14Z","labels":{"app":"nginx"},"managedFields":[{"apiVersion":"v1","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:labels":{".":{},"f:app":{}}},"f:spec":{"f:externalTrafficPolicy":{},"f:internalTrafficPolicy":{},"f:ports":{".":{},"k:{\"port\":80,\"protocol\":\"TCP\"}":{".":{},"f:port":{},"f:protocol":{},"f:targetPort":{}}},"f:selector":{},"f:sessionAffinity":{},"f:type":{}}},"manager":"kubectl-expose","operation":"Update","time":"2025-10-01T09:20:14Z"}],"name":"nginx","namespace":"default","resourceVersion":"7178","uid":"d994b815-78e4-4f42-887c-abd00ff78425"},"spec":{"clusterIP":"10.104.89.81","clusterIPs":["10.104.89.81"],"externalTrafficPolicy":"Cluster","internalTrafficPolicy":"Cluster","ipFamilies":["IPv4"],"ipFamilyPolicy":"SingleStack","ports":[{"nodePort":30369,"port":80,"protocol":"TCP","targetPort":80}],"selector":{"app":"nginx"},"sessionAffinity":"None","type":"NodePort"},"status":{"loadBalancer":{}}}}
{"type":"DELETED","object":{"apiVersion":"v1","kind":"Service","metadata":{"creationTimestamp":"2025-10-01T09:20:14Z","labels":{"app":"nginx"},"managedFields":[{"apiVersion":"v1","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:labels":{".":{},"f:app":{}}},"f:spec":{"f:externalTrafficPolicy":{},"f:internalTrafficPolicy":{},"f:ports":{".":{},"k:{\"port\":80,\"protocol\":\"TCP\"}":{".":{},"f:port":{},"f:protocol":{},"f:targetPort":{}}},"f:selector":{},"f:sessionAffinity":{},"f:type":{}}},"manager":"kubectl-expose","operation":"Update","time":"2025-10-01T09:20:14Z"}],"name":"nginx","namespace":"default","resourceVersion":"8036","uid":"d994b815-78e4-4f42-887c-abd00ff78425"},"spec":{"clusterIP":"10.104.89.81","clusterIPs":["10.104.89.81"],"externalTrafficPolicy":"Cluster","internalTrafficPolicy":"Cluster","ipFamilies":["IPv4"],"ipFamilyPolicy":"SingleStack","ports":[{"nodePort":30369,"port":80,"protocol":"TCP","targetPort":80}],"selector":{"app":"nginx"},"sessionAffinity":"None","type":"NodePort"},"status":{"loadBalancer":{}}}}
{"type":"ADDED","object":{"apiVersion":"v1","kind":"Service","metadata":{"creationTimestamp":"2025-10-01T09:37:23Z","labels":{"app":"nginx"},"managedFields":[{"apiVersion":"v1","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:labels":{".":{},"f:app":{}}},"f:spec":{"f:externalTrafficPolicy":{},"f:internalTrafficPolicy":{},"f:ports":{".":{},"k:{\"port\":80,\"protocol\":\"TCP\"}":{".":{},"f:port":{},"f:protocol":{},"f:targetPort":{}}},"f:selector":{},"f:sessionAffinity":{},"f:type":{}}},"manager":"kubectl-expose","operation":"Update","time":"2025-10-01T09:37:23Z"}],"name":"nginx","namespace":"default","resourceVersion":"8564","uid":"cf112753-1335-4edc-8374-1bfb5aceb41f"},"spec":{"clusterIP":"10.109.71.60","clusterIPs":["10.109.71.60"],"externalTrafficPolicy":"Cluster","internalTrafficPolicy":"Cluster","ipFamilies":["IPv4"],"ipFamilyPolicy":"SingleStack","ports":[{"nodePort":32762,"port":80,"protocol":"TCP","targetPort":80}],"selector":{"app":"nginx"},"sessionAffinity":"None","type":"NodePort"},"status":{"loadBalancer":{}}}}
`

	watcher := NewServiceWatcher()
	err := watcher.readKubectlStream(strings.NewReader(stream))
	assert.NilError(t, err)

	got := watcher.GetPorts()
	want := []Entry{{
		Protocol: TCP,
		IP:       net.IPv4zero,
		Port:     uint16(32762),
	}}

	assert.DeepEqual(t, got, want)
}
