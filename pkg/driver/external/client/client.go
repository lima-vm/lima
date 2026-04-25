// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"net"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/lima-vm/lima/v2/pkg/driver/external"
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
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// grpc.Dial (deprecated in favour of grpc.NewClient) is intentional here
	// for compatibility. Suppress both the staticcheck SA1019 deprecation and
	// nolintlint's unused-directive check: which staticcheck issues survive
	// past golangci-lint's filter pipeline depends on cache state, so the
	// SA1019 finding can be either kept or dropped before nolintlint inspects
	// it. Silencing nolintlint avoids a flaky lint failure across platforms.
	//nolint:staticcheck,nolintlint // grpc.Dial used for compatibility reasons
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
