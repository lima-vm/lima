// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"io"
	"math"
	"net"
	"time"

	pb "github.com/lima-vm/lima/pkg/driver/external"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

type DriverClient struct {
	Stdin     io.WriteCloser
	Stdout    io.ReadCloser
	Conn      *grpc.ClientConn
	DriverSvc pb.DriverClient
	logger    *logrus.Logger
}

func NewDriverClient(stdin io.WriteCloser, stdout io.ReadCloser, logger *logrus.Logger) (*DriverClient, error) {
	pipeConn := &PipeConn{
		Reader: stdout,
		Writer: stdin,
	}

	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(math.MaxInt64),
			grpc.MaxCallSendMsgSize(math.MaxInt64),
		),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return pipeConn, nil
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             20 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	conn, err := grpc.NewClient("pipe://", opts...)
	if err != nil {
		logger.Errorf("failed to create gRPC driver client connection: %v", err)
		return nil, err
	}

	driverSvc := pb.NewDriverClient(conn)

	return &DriverClient{
		Stdin:     stdin,
		Stdout:    stdout,
		Conn:      conn,
		DriverSvc: driverSvc,
		logger:    logger,
	}, nil
}

type PipeConn struct {
	Reader io.Reader
	Writer io.Writer
}

func (p *PipeConn) Read(b []byte) (n int, err error) {
	return p.Reader.Read(b)
}

func (p *PipeConn) Write(b []byte) (n int, err error) {
	return p.Writer.Write(b)
}

func (p *PipeConn) Close() error {
	var err error
	if closer, ok := p.Reader.(io.Closer); ok {
		err = closer.Close()
	}
	if closer, ok := p.Writer.(io.Closer); ok {
		if closeErr := closer.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}

func (p *PipeConn) LocalAddr() net.Addr {
	return pipeAddr{}
}

func (p *PipeConn) RemoteAddr() net.Addr {
	return pipeAddr{}
}

func (p *PipeConn) SetDeadline(t time.Time) error {
	return nil
}

func (p *PipeConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (p *PipeConn) SetWriteDeadline(t time.Time) error {
	return nil
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }
