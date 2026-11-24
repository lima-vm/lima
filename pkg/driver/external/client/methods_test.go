// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"gotest.tools/v3/assert"

	pb "github.com/lima-vm/lima/v2/pkg/driver/external"
)

type mockDriverClient struct {
	pb.DriverClient
	startFunc func(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (grpc.ServerStreamingClient[pb.StartResponse], error)
}

func (m *mockDriverClient) Start(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (grpc.ServerStreamingClient[pb.StartResponse], error) {
	return m.startFunc(ctx, in, opts...)
}

type mockStartStream struct {
	grpc.ServerStreamingClient[pb.StartResponse]
	recvFunc func() (*pb.StartResponse, error)
}

func (s *mockStartStream) Recv() (*pb.StartResponse, error) {
	return s.recvFunc()
}

func (s *mockStartStream) CloseSend() error {
	return nil
}

func TestStart_StreamClosesOnErrorAndEOF(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	t.Run("stream EOF closes channel without hanging", func(t *testing.T) {
		calls := 0
		mockStream := &mockStartStream{
			recvFunc: func() (*pb.StartResponse, error) {
				calls++
				if calls == 1 {
					return &pb.StartResponse{Success: true}, nil
				}
				return nil, io.EOF
			},
		}

		mockClient := &mockDriverClient{
			startFunc: func(_ context.Context, _ *emptypb.Empty, _ ...grpc.CallOption) (grpc.ServerStreamingClient[pb.StartResponse], error) {
				return mockStream, nil
			},
		}

		d := &DriverClient{
			DriverSvc: mockClient,
			logger:    logger,
		}

		errCh, err := d.Start(t.Context())
		assert.NilError(t, err)

		select {
		case err, ok := <-errCh:
			assert.Assert(t, !ok, "expected errCh to be closed on EOF")
			assert.NilError(t, err)
		case <-time.After(2 * time.Second):
			assert.Assert(t, false, "timed out waiting for errCh to close on stream EOF")
		}
	})

	t.Run("stream transport error propagates and closes channel", func(t *testing.T) {
		calls := 0
		expectedErr := errors.New("rpc connection broken")
		mockStream := &mockStartStream{
			recvFunc: func() (*pb.StartResponse, error) {
				calls++
				if calls == 1 {
					return &pb.StartResponse{Success: true}, nil
				}
				return nil, expectedErr
			},
		}

		mockClient := &mockDriverClient{
			startFunc: func(_ context.Context, _ *emptypb.Empty, _ ...grpc.CallOption) (grpc.ServerStreamingClient[pb.StartResponse], error) {
				return mockStream, nil
			},
		}

		d := &DriverClient{
			DriverSvc: mockClient,
			logger:    logger,
		}

		errCh, err := d.Start(t.Context())
		assert.NilError(t, err)

		select {
		case err, ok := <-errCh:
			assert.Assert(t, ok, "expected error received from errCh")
			assert.ErrorIs(t, err, expectedErr)
		case <-time.After(2 * time.Second):
			assert.Assert(t, false, "timed out waiting for error on stream error")
		}

		// Subsequent read should confirm channel is closed
		select {
		case _, ok := <-errCh:
			assert.Assert(t, !ok, "expected errCh to be closed after error")
		case <-time.After(1 * time.Second):
			assert.Assert(t, false, "expected errCh to close after error")
		}
	})

	t.Run("stream unsuccessful response propagates and closes channel", func(t *testing.T) {
		calls := 0
		mockStream := &mockStartStream{
			recvFunc: func() (*pb.StartResponse, error) {
				calls++
				if calls == 1 {
					return &pb.StartResponse{Success: true}, nil
				}
				return &pb.StartResponse{Success: false, Error: "boot timeout"}, nil
			},
		}

		mockClient := &mockDriverClient{
			startFunc: func(_ context.Context, _ *emptypb.Empty, _ ...grpc.CallOption) (grpc.ServerStreamingClient[pb.StartResponse], error) {
				return mockStream, nil
			},
		}

		d := &DriverClient{
			DriverSvc: mockClient,
			logger:    logger,
		}

		errCh, err := d.Start(t.Context())
		assert.NilError(t, err)

		select {
		case err, ok := <-errCh:
			assert.Assert(t, ok, "expected error received from errCh")
			assert.ErrorContains(t, err, "boot timeout")
		case <-time.After(2 * time.Second):
			assert.Assert(t, false, "timed out waiting for error on stream failure")
		}

		select {
		case _, ok := <-errCh:
			assert.Assert(t, !ok, "expected errCh to be closed after error")
		case <-time.After(1 * time.Second):
			assert.Assert(t, false, "expected errCh to close after error")
		}
	})
}
