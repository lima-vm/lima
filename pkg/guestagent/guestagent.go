package guestagent

import (
	"context"

	"github.com/AkihiroSuda/lima/pkg/guestagent/api"
)

type Agent interface {
	Info(ctx context.Context) (*api.Info, error)
	Events(ctx context.Context, ch chan api.Event)
	LocalPorts(ctx context.Context) ([]api.IPPort, error)
}
