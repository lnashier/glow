package flow

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	"github.com/lnashier/glow/help"
	"github.com/lnashier/goarc"
	"slices"
)

type Flow struct {
	net       *glow.Network
	keygen    func() string
	preStep   *Step
	err       error
	callbacks []func()
}

func New(opt ...glow.NetworkOpt) *Flow {
	return &Flow{
		net:    glow.New(opt...),
		keygen: Keygen("node"),
	}
}

// Read retrieves data from a specified source using a provided reader function.
// The reader function is called with a context and an emit function, responsible for
// reading data and emitting it. The emitted data can be of any type.
// Usually, this is the first step, feeding data for processing on to subsequent steps.
func (f *Flow) Read(rf func(ctx context.Context, emit func(any)) error) *Flow {
	return f.seed(ReadStep, rf)
}

// Map applies a mapping function to each element in the input data stream.
// The mapper function is invoked with a context, an input element, and an emit function.
// It processes each input element and emits zero or more transformed data points using the 'emit' function.
// The emitted data can be of any type.
// Typically, this step is an intermediate step enabling data transformation operations.
func (f *Flow) Map(mf func(ctx context.Context, in any, emit func(any)) error) *Flow {
	return f.transit(MapStep, mf)
}

// Peek allows observing the data stream without modifying it, typically
// for debugging, logging, or monitoring purposes.
func (f *Flow) Peek(pf func(in any)) *Flow {
	return f.transit(PeekStep, func(ctx context.Context, in any, emit func(any)) error {
		pf(in)
		emit(in)
		return nil
	})
}

// Filter applies a filtering function to each element in the input data stream.
// The filtering function is invoked with an input element and returns a boolean
// indicating whether the element should be retained or not.
// If the filtering function returns true, the element is passed through to the output stream;
// otherwise, it is discarded.
// This step serves as an intermediate step facilitating data filtering operations.
func (f *Flow) Filter(ff func(in any) bool) *Flow {
	return f.transit(FilterStep, func(ctx context.Context, in any, emit func(any)) error {
		if ff(in) {
			select {
			case <-ctx.Done():
				// return immediately
				return nil
			default:
				emit(in)
			}
		}
		return nil
	})
}

// Capture captures each element in the input data stream and feeds it to a capturing function.
// The capturing function receives a context and captured data point.
// Being a terminal step in the pipeline, it does not emit data.
func (f *Flow) Capture(cf func(ctx context.Context, in any) error) *Flow {
	return f.terminal(CaptureStep, cf)
}

// Collect aggregates all elements in the input data stream and provides the collection to provided callback.
// It accepts the following options:
//   - Compare option allows sorting the collection before passing it to the callback.
func (f *Flow) Collect(cb func([]any), opt ...Opt) *Flow {
	nodeOpts := &opts{}
	nodeOpts.apply(opt...)

	var tokens []any

	f.terminal(CollectStep, func(ctx context.Context, in any) error {
		for i, token := range tokens {
			comp := nodeOpts.compare(token, in)
			if comp > 0 {
				tokens = append(tokens[:i+1], tokens[i:]...)
				tokens[i] = in
				return nil
			}
		}
		tokens = append(tokens, in)
		return nil
	})

	f.callbacks = append(f.callbacks, func() {
		cb(tokens)
	})

	return f
}

// Count counts the number of elements in the input data stream.
func (f *Flow) Count(cb func(num int)) *Flow {
	var tokens []any
	f.terminal(CountStep, func(ctx context.Context, in any) error {
		tokens = append(tokens, in)
		return nil
	})
	f.callbacks = append(f.callbacks, func() {
		cb(len(tokens))
	})
	return f
}

// Run initiates the processing of data through the pipeline, starting from the
// initial input source and sequentially applying each operation
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
	for _, callback := range f.callbacks {
		callback()
	}
	return f
}

// Error retrieves any error that occurred during the building
// and execution of the pipeline.
func (f *Flow) Error() error {
	return f.err
}

// Draw generates a Graphviz visualization of the Flow.
func (f *Flow) Draw(name string) *Flow {
	f.appendError(help.Draw(f.net, name))
	return f
}

func (f *Flow) seed(kind StepKind, sf func(ctx context.Context, emit func(any)) error) *Flow {
	nodeID, err := f.net.AddNode(
		glow.EmitterFunc(func(ctx context.Context, _ any, emit func(any)) error {
			return sf(ctx, emit)
		}),
		glow.KeyFunc(f.keygen),
	)
	f.appendError(err)
	f.link(&Step{
		id:   nodeID,
		kind: kind,
	})
	return f
}

func (f *Flow) transit(kind StepKind, tf func(ctx context.Context, in any, emit func(any)) error) *Flow {
	nodeID, err := f.net.AddNode(
		glow.EmitterFunc(func(ctx context.Context, in any, emit func(any)) error {
			return tf(ctx, in, emit)
		}),
		glow.KeyFunc(f.keygen),
	)
	f.appendError(err)
	f.link(&Step{
		id:   nodeID,
		kind: kind,
	})
	return f
}

func (f *Flow) terminal(kind StepKind, tf func(ctx context.Context, in any) error) *Flow {
	nodeID, err := f.net.AddNode(
		glow.NodeFunc(func(ctx context.Context, in any) (any, error) {
			return nil, tf(ctx, in)
		}),
		glow.KeyFunc(f.keygen),
	)
	f.appendError(err)
	f.link(&Step{
		id:   nodeID,
		kind: kind,
	})
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

func (f *Flow) link(step *Step) {
	if f.preStep != nil {
		if slices.Contains(seedKinds, step.kind) {
			f.appendError(fmt.Errorf("can not add %s step after %s step", step.kind, f.preStep.kind))
			return
		}
		if slices.Contains(terminalKinds, f.preStep.kind) {
			f.appendError(fmt.Errorf("can not add %s step after %s step", step.kind, f.preStep.kind))
			return
		}
		f.appendError(f.net.AddLink(f.preStep.id, step.id))
	}
	f.preStep = step
}

type Opt func(*opts)

type opts struct {
	compare func(any, any) int
}

func (o *opts) apply(opt ...Opt) {
	for _, op := range opt {
		op(o)
	}
}

// Compare function must return
//
//	< 0 if a is less than b,
//	= 0 if a equals b,
//	> 0 if a is greater than b.
func Compare(f func(a any, b any) int) Opt {
	return func(o *opts) {
		o.compare = f
	}
}
