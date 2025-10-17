// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func TestResolvePortForwardTypes(t *testing.T) {
	tests := []struct {
		name             string
		portForwardTypes map[limatype.Proto]limatype.PortForwardType
		portForwards     []limatype.PortForward
		env              map[string]string
		expected         map[limatype.Proto]limatype.PortForwardType
		expectedErr      string
	}{
		{
			name: "default",
			portForwardTypes: map[limatype.Proto]limatype.PortForwardType{
				limatype.ProtoTCP: limatype.PortForwardTypeSSH,
				limatype.ProtoUDP: limatype.PortForwardTypeGRPC,
			},
			portForwards: nil,
			env:          nil,
			expected: map[limatype.Proto]limatype.PortForwardType{
				limatype.ProtoTCP: limatype.PortForwardTypeSSH,
				limatype.ProtoUDP: limatype.PortForwardTypeGRPC,
			},
		},
		{
			name: "grpc only via config",
			portForwardTypes: map[limatype.Proto]limatype.PortForwardType{
				limatype.ProtoAny: limatype.PortForwardTypeGRPC,
			},
			portForwards: nil,
			env:          nil,
			expected: map[limatype.Proto]limatype.PortForwardType{
				limatype.ProtoTCP: limatype.PortForwardTypeGRPC,
				limatype.ProtoUDP: limatype.PortForwardTypeGRPC,
			},
		},
		{
			name: "ssh only via env",
			portForwardTypes: map[limatype.Proto]limatype.PortForwardType{
				limatype.ProtoTCP: limatype.PortForwardTypeSSH,
				limatype.ProtoUDP: limatype.PortForwardTypeGRPC,
			},
			portForwards: nil,
			env: map[string]string{
				"LIMA_SSH_PORT_FORWARDER": "true",
			},
			expected: map[limatype.Proto]limatype.PortForwardType{
				limatype.ProtoTCP: limatype.PortForwardTypeSSH,
				// No UDP support in SSH port forwarder
			},
		},
		{
			name: "grpc only via env",
			portForwardTypes: map[limatype.Proto]limatype.PortForwardType{
				limatype.ProtoTCP: limatype.PortForwardTypeSSH,
				limatype.ProtoUDP: limatype.PortForwardTypeGRPC,
			},
			portForwards: nil,
			env: map[string]string{
				"LIMA_SSH_PORT_FORWARDER": "false",
			},
			expected: map[limatype.Proto]limatype.PortForwardType{
				limatype.ProtoTCP: limatype.PortForwardTypeGRPC,
				limatype.ProtoUDP: limatype.PortForwardTypeGRPC,
			},
		},
		{
			name: "disable tcp via portForwards",
			portForwardTypes: map[limatype.Proto]limatype.PortForwardType{
				limatype.ProtoTCP: limatype.PortForwardTypeSSH,
				limatype.ProtoUDP: limatype.PortForwardTypeGRPC,
			},
			portForwards: []limatype.PortForward{
				{
					Ignore:         true,
					Proto:          limatype.ProtoTCP,
					GuestPortRange: [2]int{1, 65535},
				},
			},
			env: nil,
			expected: map[limatype.Proto]limatype.PortForwardType{
				limatype.ProtoUDP: limatype.PortForwardTypeGRPC,
			},
		},
		{
			name: "disable tcp via portForwards, with any in portForwardTypes",
			portForwardTypes: map[limatype.Proto]limatype.PortForwardType{
				limatype.ProtoAny: limatype.PortForwardTypeGRPC,
			},
			portForwards: []limatype.PortForward{
				{
					Ignore:         true,
					Proto:          limatype.ProtoTCP,
					GuestPortRange: [2]int{1, 65535},
				},
			},
			env: nil,
			expected: map[limatype.Proto]limatype.PortForwardType{
				limatype.ProtoUDP: limatype.PortForwardTypeGRPC,
			},
		},
		{
			name: "conflict between any and tcp",
			portForwardTypes: map[limatype.Proto]limatype.PortForwardType{
				limatype.ProtoAny: limatype.PortForwardTypeGRPC,
				limatype.ProtoTCP: limatype.PortForwardTypeSSH,
			},
			portForwards: nil,
			env:          nil,
			expectedErr:  "conflicting port forward types for proto",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for envK, envV := range tt.env {
				t.Setenv(envK, envV)
			}
			got, err := resolvePortForwardTypes(tt.portForwardTypes, tt.portForwards)
			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
				return
			}
			assert.NilError(t, err)
			assert.DeepEqual(t, got, tt.expected)
		})
	}
}
