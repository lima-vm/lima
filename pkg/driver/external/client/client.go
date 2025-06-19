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
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(16 << 20)),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(16 << 20)),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

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
