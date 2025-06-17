// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"net"

	pb "github.com/lima-vm/lima/pkg/driver/external"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type DriverClient struct {
	socketPath string
	Conn       *grpc.ClientConn
	DriverSvc  pb.DriverClient
	logger     *logrus.Logger
}

func NewDriverClient(socketPath string, logger *logrus.Logger) (*DriverClient, error) {
	// pipeConn := newPipeConn(stdin, stdout)
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(16 << 20)),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(16 << 20)),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// conn, err := grpc.NewClient("pipe", opts...)
	// if err != nil {
	// 	logger.Errorf("failed to create gRPC driver client connection: %v", err)
	// 	return nil, err
	// }
	// -> ERRO[2025-06-04T21:32:54+05:30] Failed to set config: rpc error: code =
	// Unavailable desc = name resolver error: produced zero addresses

	conn, err := grpc.Dial("unix://"+socketPath, opts...)
	if err != nil {
		logger.Errorf("failed to dial gRPC driver client connection: %v", err)
		return nil, err
	}

	driverSvc := pb.NewDriverClient(conn)

	return &DriverClient{
		socketPath: socketPath,
		Conn:       conn,
		DriverSvc:  driverSvc,
		logger:     logger,
	}, nil
}
