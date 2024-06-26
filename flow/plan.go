package flow

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	"github.com/lnashier/glow/help"
	"slices"
	"sync"
	"time"
)

type Plan struct {
	net       *glow.Network
	once      *sync.Once
	opts      []*stepOpts
	err       error
	callbacks []func()
}

func New(opt ...glow.NetworkOpt) *Plan {
	return &Plan{
		net:  glow.New(opt...),
		once: &sync.Once{},
	}
}

func (p *Plan) Step(opt ...StepOpt) *Plan {
	p.opts = append(p.opts, (&stepOpts{}).apply(opt...))
	return p
}

func (p *Plan) Run(ctx context.Context) *Plan {
	p.build()
	if p.err == nil {
		p.appendError(p.net.Start(ctx))
		if p.err == nil {
			for _, callback := range p.callbacks {
				callback()
			}
		}
	}
	return p
}

func (p *Plan) Stop() *Plan {
	if p.err == nil {
		p.appendError(p.net.Stop())
	}
	return p
}

func (p *Plan) Draw(name string) *Plan {
	p.build()
	p.appendError(help.Draw(p.net, name))
	return p
}

func (p *Plan) Uptime(uf func(d time.Duration)) *Plan {
	uf(p.net.Uptime())
	return p
}

// Error retrieves any error that occurred during the building and execution of the pipeline.
func (p *Plan) Error() error {
	return p.err
}

func (p *Plan) build() {
	p.once.Do(func() {
		replicaKeygen := func(key string, r int) string {
			return fmt.Sprintf("%s-r%d", key, r)
		}

		steps := make(map[string][]*Step)

		// make nodes
		for _, opts := range p.opts {
			if opts.replicas < 1 {
				opts.replicas = 1
			}

			if slices.Contains(linearKinds, opts.kind) && opts.replicas != 1 {
				p.appendError(fmt.Errorf("%s step concurrency != 1", opts.kind))
			}

			var replicas []*Step

			for i := range opts.replicas {
				replicaKey := opts.key
				if opts.replicas > 1 && len(opts.key) > 0 {
					replicaKey = replicaKeygen(opts.key, i+1)
				}
				nodeOpts := []glow.NodeOpt{
					glow.Key(replicaKey),
					glow.EmitFunc(opts.sf),
				}
				if opts.distributor {
					nodeOpts = append(nodeOpts, glow.Distributor())
				}
				nodeID, err := p.net.AddNode(nodeOpts...)
				p.appendError(err)
				if err == nil {
					replicas = append(replicas, &Step{
						id:   nodeID,
						kind: opts.kind,
					})
				}
			}

			steps[opts.key] = replicas

			if opts.callback != nil && p.Error() == nil {
				p.callbacks = append(p.callbacks, opts.callback)
			}
		}

		// make links
		for _, y := range p.opts {
			for _, yReplica := range steps[y.key] {
				for _, x := range y.connections {
					xReplicas := steps[x]
					if len(xReplicas) < 1 {
						p.appendError(fmt.Errorf("%s connecting to unknown %s", y.key, x))
						continue
					}
					for _, xReplica := range xReplicas {
						p.appendError(p.net.AddLink(xReplica.id, yReplica.id))
					}
				}
			}
		}
	})
}

func (p *Plan) appendError(err error) {
	if err != nil {
		if p.err != nil {
			p.err = fmt.Errorf("%w %w", p.err, err)
		} else {
			p.err = err
		}
	}
}
