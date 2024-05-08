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
		net:    glow.New(glow.PreventCycles(), glow.Verbose()),
		keygen: Keygen("node"),
	}
}

// Read retrieves data from a specified source using a provided reader function.
// The reader function is called with a context and an emit function, responsible for
// reading data and emitting it. The emitted data can be of any type.
// Usually, this is the first step, feeding data for processing on to subsequent steps.
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

// Map applies a mapping function to each element in the input data stream.
// The mapper function is invoked with a context, an input element, and an emit function.
// It processes each input element and emits zero or more transformed data points using the 'emit' function.
// The emitted data can be of any type.
// Typically, this step is an intermediate step enabling data transformation operations.
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

// Filter applies a filtering function to each element in the input data stream.
// The filtering function is invoked with an input element and returns a boolean
// indicating whether the element should be retained or not.
// If the filtering function returns true, the element is passed through to the output stream;
// otherwise, it is discarded.
// This step serves as an intermediate step facilitating data filtering operations.
func (f *Flow) Filter(ff func(in any) bool) *Flow {
	nodeID, err := f.net.AddNode(
		glow.EmitterFunc(func(ctx context.Context, in any, emit func(any)) error {
			if ff(in) {
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

// Capture captures each element in the input data stream and feeds it to a capturing function.
// The capturing function receives a context and captured data point.
// Being a terminal step in the pipeline, it does not emit data.
func (f *Flow) Capture(cf func(ctx context.Context, in any) error) *Flow {
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

// Run initiates the processing of data through the pipeline, starting from the
// initial input source and sequentially applying each transformation or operation
// defined in the pipeline until reaching the terminal step.
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

// Error retrieves any error that occurred during the building and execution of the pipeline.
func (f *Flow) Error() error {
	return f.err
}

// Draw generates a Graphviz (GV) visualization of the pipeline
func (f *Flow) Draw(name string) *Flow {
	f.appendError(help.Draw(f.net, name))
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
