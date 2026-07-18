// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwdserver

import (
	"bytes"
	"io"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

// recvOnlyTunnelServer implements only Recv of the bidi stream; the other
// methods are never called by GRPCServerRW.Read.
type recvOnlyTunnelServer struct {
	api.GuestService_TunnelServer
	msgs []*api.TunnelMessage
}

func (s *recvOnlyTunnelServer) Recv() (*api.TunnelMessage, error) {
	if len(s.msgs) == 0 {
		return nil, io.EOF
	}
	msg := s.msgs[0]
	s.msgs = s.msgs[1:]
	return msg, nil
}

func TestGRPCServerRWReadShortBuffer(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 100)
	rw := &GRPCServerRW{stream: &recvOnlyTunnelServer{msgs: []*api.TunnelMessage{{Data: payload}}}}

	var got []byte
	buf := make([]byte, 10)
	for len(got) < len(payload) {
		n, err := rw.Read(buf)
		assert.NilError(t, err)
		assert.Assert(t, n <= len(buf), "Read returned %d, larger than the %d-byte buffer", n, len(buf))
		got = append(got, buf[:n]...)
	}
	assert.DeepEqual(t, got, payload)
}
