package mapreduce

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	"github.com/lnashier/glow/help"
	"github.com/lnashier/goarc"
)

type Flow struct {
	net          *glow.Network
	keygen       func() string
	previousNode string
	err          error
}

func New() *Flow {
	return &Flow{
		net:    glow.New(glow.PreventCycles()),
		keygen: Keygen("node"),
	}
}

func (f *Flow) Run() *Flow {
	if f.err != nil {
		return f
	}
	goarc.Up(
		f.net,
		goarc.OnStart(func(err error) {
			f.appendError(err)
		}),
		goarc.OnStop(func(err error) {
			f.appendError(err)
		}),
	)
	return f
}

func (f *Flow) Error() error {
	return f.err
}

func (f *Flow) Draw(name string) *Flow {
	f.appendError(help.Draw(f.net, name))
	return f
}

func (f *Flow) Read(rf func(ctx context.Context, emit func(any)) error) *Flow {
	nodeID, err := f.net.AddNode(
		glow.EmitterFunc(func(ctx context.Context, _ any, emit func(any)) error {
			return rf(ctx, emit)
		}),
		glow.KeyFunc(f.keygen),
	)
	f.appendError(err)
	f.link(nodeID)
	return f
}

func (f *Flow) Map(mf func(ctx context.Context, in any, emit func(any)) error) *Flow {
	nodeID, err := f.net.AddNode(
		glow.EmitterFunc(func(ctx context.Context, in any, emit func(any)) error {
			return mf(ctx, in, emit)
		}),
		glow.KeyFunc(f.keygen),
	)
	f.appendError(err)
	f.link(nodeID)
	return f
}

func (f *Flow) Filter(ff func(in any) bool) *Flow {
	nodeID, err := f.net.AddNode(
		glow.EmitterFunc(func(ctx context.Context, in any, emit func(any)) error {
			if !ff(in) {
				select {
				case <-ctx.Done():
					return nil
				default:
					emit(in)
				}
			}
			return nil
		}),
		glow.KeyFunc(f.keygen),
	)
	f.appendError(err)
	f.link(nodeID)
	return f
}

func (f *Flow) Collect(cf func(ctx context.Context, in any) error) *Flow {
	nodeID, err := f.net.AddNode(
		glow.NodeFunc(func(ctx context.Context, in any) (any, error) {
			return nil, cf(ctx, in)
		}),
		glow.KeyFunc(f.keygen),
	)
	f.appendError(err)
	f.link(nodeID)
	return f
}

func (f *Flow) appendError(err error) {
	if err != nil {
		if f.err != nil {
			f.err = fmt.Errorf("%w %w", f.err, err)
		} else {
			f.err = err
		}
	}
}

func (f *Flow) link(nodeID string) {
	if f.previousNode != "" {
		f.appendError(f.net.AddLink(f.previousNode, nodeID))
	}
	f.previousNode = nodeID
}
