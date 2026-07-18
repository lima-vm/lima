// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package kubernetesservice

// The type definitions were taken from https://pkg.go.dev/k8s.io/api@v0.34.3/core/v1
// to reduce Go module dependencies.
// SPDX-FileCopyrightText: Copyright 2015 The Kubernetes Authors.

type service struct {
	Metadata objectMeta  `json:"metadata"`
	Spec     serviceSpec `json:"spec"`
}

type objectMeta struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type serviceSpec struct {
	Type  serviceType   `json:"type"`
	Ports []servicePort `json:"ports"`
}

type servicePort struct {
	Name       string      `json:"name"`
	Protocol   k8sProtocol `json:"protocol"`
	Port       int32       `json:"port"`
	TargetPort any         `json:"targetPort"` // int or string
	NodePort   int32       `json:"nodePort"`
}

type serviceType string

const (
	serviceTypeNodePort     serviceType = "NodePort"
	serviceTypeLoadBalancer serviceType = "LoadBalancer"
)

type k8sProtocol string

const (
	protocolTCP k8sProtocol = "TCP"
	protocolUDP k8sProtocol = "UDP"
)

type eventType string

const (
	added    eventType = "ADDED"
	modified eventType = "MODIFIED"
	deleted  eventType = "DELETED"
)
