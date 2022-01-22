// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Package client implements client.
package client

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gyuho/avax-tester/pkg/color"
	"github.com/gyuho/avax-tester/pkg/logutil"
	"github.com/gyuho/avax-tester/rpcpb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type Config struct {
	LogLevel    string
	Endpoint    string
	DialTimeout time.Duration
}

type Client interface {
	Ping(ctx context.Context) (*rpcpb.PingResponse, error)
	Start(ctx context.Context, execPath string, opts ...OpOption) (*rpcpb.ClusterInfo, error)
	Health(ctx context.Context) (*rpcpb.ClusterInfo, error)
	URIs(ctx context.Context) ([]string, error)
	Status(ctx context.Context) (*rpcpb.ClusterInfo, error)
	StreamStatus(ctx context.Context, pushInterval time.Duration) (<-chan *rpcpb.ClusterInfo, error)
	RemoveNode(ctx context.Context, name string) (*rpcpb.ClusterInfo, error)
	RestartNode(ctx context.Context, name string, execPath string, opts ...OpOption) (*rpcpb.ClusterInfo, error)
	Stop(ctx context.Context) (*rpcpb.ClusterInfo, error)
	Close() error
}

type client struct {
	cfg Config

	conn *grpc.ClientConn

	pingc    rpcpb.PingServiceClient
	controlc rpcpb.ControlServiceClient

	closed    chan struct{}
	closeOnce sync.Once
}

func New(cfg Config) (Client, error) {
	lcfg := logutil.GetDefaultZapLoggerConfig()
	lcfg.Level = zap.NewAtomicLevelAt(logutil.ConvertToZapLevel(cfg.LogLevel))
	logger, err := lcfg.Build()
	if err != nil {
		return nil, err
	}
	_ = zap.ReplaceGlobals(logger)

	color.Outf("{{blue}}dialing endpoint %q{{/}}\n", cfg.Endpoint)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	conn, err := grpc.DialContext(
		ctx,
		cfg.Endpoint,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	cancel()
	if err != nil {
		return nil, err
	}

	return &client{
		cfg:      cfg,
		conn:     conn,
		pingc:    rpcpb.NewPingServiceClient(conn),
		controlc: rpcpb.NewControlServiceClient(conn),
		closed:   make(chan struct{}),
	}, nil
}

func (c *client) Ping(ctx context.Context) (*rpcpb.PingResponse, error) {
	zap.L().Info("ping")

	// ref. https://grpc-ecosystem.github.io/grpc-gateway/docs/tutorials/adding_annotations/
	// curl -X POST -k http://localhost:8081/v1/ping -d ''
	return c.pingc.Ping(ctx, &rpcpb.PingRequest{})
}

func (c *client) Start(ctx context.Context, execPath string, opts ...OpOption) (*rpcpb.ClusterInfo, error) {
	ret := &Op{}
	ret.applyOpts(opts)

	zap.L().Info("start")
	resp, err := c.controlc.Start(ctx, &rpcpb.StartRequest{
		ExecPath:           execPath,
		WhitelistedSubnets: &ret.whitelistedSubnets,
	})
	if err != nil {
		return nil, err
	}
	return resp.ClusterInfo, nil
}

func (c *client) Health(ctx context.Context) (*rpcpb.ClusterInfo, error) {
	zap.L().Info("health")
	resp, err := c.controlc.Health(ctx, &rpcpb.HealthRequest{})
	if err != nil {
		return nil, err
	}
	return resp.ClusterInfo, nil
}

func (c *client) URIs(ctx context.Context) ([]string, error) {
	zap.L().Info("uris")
	resp, err := c.controlc.URIs(ctx, &rpcpb.URIsRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Uris, nil
}

func (c *client) Status(ctx context.Context) (*rpcpb.ClusterInfo, error) {
	zap.L().Info("status")
	resp, err := c.controlc.Status(ctx, &rpcpb.StatusRequest{})
	if err != nil {
		return nil, err
	}
	return resp.ClusterInfo, nil
}

func (c *client) StreamStatus(ctx context.Context, pushInterval time.Duration) (<-chan *rpcpb.ClusterInfo, error) {
	stream, err := c.controlc.StreamStatus(ctx, &rpcpb.StreamStatusRequest{
		PushInterval: int64(pushInterval),
	})
	if err != nil {
		return nil, err
	}

	ch := make(chan *rpcpb.ClusterInfo, 1)
	go func() {
		defer func() {
			zap.L().Debug("closing stream send", zap.Error(stream.CloseSend()))
			close(ch)
		}()
		zap.L().Info("start receive routine")
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.closed:
				return
			default:
			}

			// receive data from stream
			msg := new(rpcpb.StatusResponse)
			err := stream.RecvMsg(msg)
			if err == nil {
				ch <- msg.GetClusterInfo()
				continue
			}

			if errors.Is(err, io.EOF) {
				zap.L().Debug("received EOF from client; returning to close the stream from server side")
				return
			}
			if isClientCanceled(stream.Context().Err(), err) {
				zap.L().Warn("failed to receive status request from gRPC stream due to client cancellation", zap.Error(err))
			} else {
				zap.L().Warn("failed to receive status request from gRPC stream", zap.Error(err))
			}
			return
		}
	}()
	return ch, nil
}

func (c *client) Stop(ctx context.Context) (*rpcpb.ClusterInfo, error) {
	zap.L().Info("stop")
	resp, err := c.controlc.Stop(ctx, &rpcpb.StopRequest{})
	if err != nil {
		return nil, err
	}
	return resp.ClusterInfo, nil
}

func (c *client) RemoveNode(ctx context.Context, name string) (*rpcpb.ClusterInfo, error) {
	zap.L().Info("remove node", zap.String("name", name))
	resp, err := c.controlc.RemoveNode(ctx, &rpcpb.RemoveNodeRequest{Name: name})
	if err != nil {
		return nil, err
	}
	return resp.ClusterInfo, nil
}

func (c *client) RestartNode(ctx context.Context, name string, execPath string, opts ...OpOption) (*rpcpb.ClusterInfo, error) {
	ret := &Op{}
	ret.applyOpts(opts)

	zap.L().Info("restart node", zap.String("name", name))
	resp, err := c.controlc.RestartNode(ctx, &rpcpb.RestartNodeRequest{
		Name: name,
		StartRequest: &rpcpb.StartRequest{
			ExecPath:           execPath,
			WhitelistedSubnets: &ret.whitelistedSubnets,
		},
	})
	if err != nil {
		return nil, err
	}
	return resp.ClusterInfo, nil
}

func (c *client) Close() error {
	c.closeOnce.Do(func() {
		close(c.closed)
	})
	return c.conn.Close()
}

type Op struct {
	whitelistedSubnets string
}

type OpOption func(*Op)

func (op *Op) applyOpts(opts []OpOption) {
	for _, opt := range opts {
		opt(op)
	}
}

func WithWhitelistedSubnets(whitelistedSubnets string) OpOption {
	return func(op *Op) {
		op.whitelistedSubnets = whitelistedSubnets
	}
}

func isClientCanceled(ctxErr error, err error) bool {
	if ctxErr != nil {
		return true
	}

	ev, ok := status.FromError(err)
	if !ok {
		return false
	}

	switch ev.Code() {
	case codes.Canceled, codes.DeadlineExceeded:
		// client-side context cancel or deadline exceeded
		// "rpc error: code = Canceled desc = context canceled"
		// "rpc error: code = DeadlineExceeded desc = context deadline exceeded"
		return true
	case codes.Unavailable:
		msg := ev.Message()
		// client-side context cancel or deadline exceeded with TLS ("http2.errClientDisconnected")
		// "rpc error: code = Unavailable desc = client disconnected"
		if msg == "client disconnected" {
			return true
		}
		// "grpc/transport.ClientTransport.CloseStream" on canceled streams
		// "rpc error: code = Unavailable desc = stream error: stream ID 21; CANCEL")
		if strings.HasPrefix(msg, "stream error: ") && strings.HasSuffix(msg, "; CANCEL") {
			return true
		}
	}
	return false
}
