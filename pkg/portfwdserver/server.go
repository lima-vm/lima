package portfwdserver

import (
	"errors"
	"io"
	"net"

	"github.com/lima-vm/lima/pkg/guestagent/api"
)

type TunnelServer struct {
	Conns map[string]net.Conn
}

func NewTunnelServer() *TunnelServer {
	return &TunnelServer{
		Conns: make(map[string]net.Conn),
	}
}

func (s *TunnelServer) Start(stream api.GuestService_TunnelServer) error {
	for {
		in, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if len(in.Data) == 0 {
			continue
		}

		conn, ok := s.Conns[in.Id]
		if !ok {
			conn, err = net.Dial(in.Protocol, in.GuestAddr)
			if err != nil {
				return err
			}
			s.Conns[in.Id] = conn

			writer := &GRPCServerWriter{id: in.Id, udpAddr: in.UdpTargetAddr, stream: stream}
			go func() {
				_, _ = io.Copy(writer, conn)
				delete(s.Conns, writer.id)
			}()
		}
		_, err = conn.Write(in.Data)
		if err != nil {
			return err
		}
	}
}

type GRPCServerWriter struct {
	id      string
	udpAddr string
	stream  api.GuestService_TunnelServer
}

var _ io.Writer = (*GRPCServerWriter)(nil)

func (g GRPCServerWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	err = g.stream.Send(&api.TunnelMessage{Id: g.id, Data: p, UdpTargetAddr: g.udpAddr})
	return len(p), err
}
