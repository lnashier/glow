package flow

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	"github.com/lnashier/glow/help"
	"slices"
	"time"
)

type Seq struct {
	net       *glow.Network
	keygen    func() string
	err       error
	callbacks []func()
	preSteps  []*Step
}

func Sequential(opt ...glow.NetworkOpt) *Seq {
	opt = append(opt, glow.PreventCycles())
	return &Seq{
		net:    glow.New(opt...),
		keygen: keygen("node"),
	}
}

// Read retrieves data from a specified source using a provided reader function.
// The reader function is called with a context and an emit function, responsible for
// reading data and emitting it. The emitted data can be of any type.
// Usually, this is the first step, feeding data for processing on to subsequent steps.
func (s *Seq) Read(rf func(ctx context.Context, emit func(any)) error, opt ...StepOpt) *Seq {
	s.seed(ReadStep, rf, (&stepOpts{}).apply(opt...))
	return s
}

// Map applies a mapping function to each element in the input data stream.
// The mapper function is invoked with a context, an input element, and an emit function.
// It processes each input element and emits zero or more transformed data points using the 'emit' function.
// The emitted data can be of any type.
// Typically, this step is an intermediate step enabling data transformation operations.
func (s *Seq) Map(mf func(ctx context.Context, in any, emit func(any)) error, opt ...StepOpt) *Seq {
	s.transit(MapStep, mf, (&stepOpts{}).apply(opt...))
	return s
}

// Peek allows observing the data stream without modifying it, typically
// for debugging, logging, or monitoring purposes.
func (s *Seq) Peek(pf func(in any), opt ...StepOpt) *Seq {
	s.transit(PeekStep, func(ctx context.Context, in any, emit func(any)) error {
		pf(in)
		emit(in)
		return nil
	}, (&stepOpts{}).apply(opt...))
	return s
}

// Combine combines the elements of streams into a single stream, concatenating
// all the data points from the individual streams in the order they arrive.
func (s *Seq) Combine(opt ...StepOpt) *Seq {
	s.transit(CombineStep, func(ctx context.Context, in any, emit func(any)) error {
		emit(in)
		return nil
	}, (&stepOpts{}).apply(opt...))
	return s
}

// Filter applies a filtering function to each element in the input data stream.
// The filtering function is invoked with an input element and returns a boolean
// indicating whether the element should be retained or not.
// If the filtering function returns true, the element is passed through to the output stream;
// otherwise, it is discarded.
// This step serves as an intermediate step facilitating data filtering operations.
func (s *Seq) Filter(ff func(in any) bool, opt ...StepOpt) *Seq {
	s.transit(FilterStep, func(ctx context.Context, in any, emit func(any)) error {
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
	}, (&stepOpts{}).apply(opt...))
	return s
}

// Capture captures each element in the input data stream and feeds it to a capturing function.
// The capturing function receives a context and captured data point.
// Being a terminal step in the pipeline, it does not emit data.
func (s *Seq) Capture(cf func(ctx context.Context, in any) error, opt ...StepOpt) *Seq {
	s.terminal(CaptureStep, cf, (&stepOpts{}).apply(opt...))
	return s
}

// Collect aggregates all elements in the input data stream and provides the collection to the provided callback.
// It accepts the Compare option to allow sorting the collected data points as they arrive.
// As a terminal step in the pipeline, it does not emit data, marking the end of the data processing flow.
func (s *Seq) Collect(cb func([]any), opt ...StepOpt) *Seq {
	opts := &stepOpts{
		compare: func(a any, b any) int {
			return 0
		},
	}
	opts.apply(opt...)

	var tokens []any
	s.callbacks = append(s.callbacks, func() {
		cb(tokens)
	})

	s.terminal(CollectStep, func(ctx context.Context, in any) error {
		for i, token := range tokens {
			comp := opts.compare(token, in)
			if comp > 0 {
				tokens = append(tokens[:i+1], tokens[i:]...)
				tokens[i] = in
				return nil
			}
		}
		tokens = append(tokens, in)
		return nil
	}, opts)

	return s
}

// Count keeps track of the number of elements in the input data stream.
// Being a terminal step in the pipeline, it does not emit data.
func (s *Seq) Count(cb func(num int), opt ...StepOpt) *Seq {
	var tokens []any
	s.callbacks = append(s.callbacks, func() {
		cb(len(tokens))
	})

	s.terminal(CountStep, func(ctx context.Context, in any) error {
		tokens = append(tokens, in)
		return nil
	}, (&stepOpts{}).apply(opt...))

	return s
}

// Run initiates the processing of data through the pipeline, starting from the
// initial input source and sequentially applying each operation
// defined in the pipeline until reaching the terminal step.
func (s *Seq) Run(ctx context.Context) *Seq {
	if s.err == nil {
		s.appendError(s.net.Start(ctx))
		if s.err == nil {
			for _, callback := range s.callbacks {
				callback()
			}
		}
	}
	return s
}

func (s *Seq) Stop() *Seq {
	if s.err == nil {
		s.appendError(s.net.Stop())
	}
	return s
}

// Draw generates a Graphviz visualization of the Flow.
func (s *Seq) Draw(name string) *Seq {
	s.appendError(help.Draw(s.net, name))
	return s
}

func (s *Seq) Uptime(uf func(d time.Duration)) *Seq {
	uf(s.net.Uptime())
	return s
}

// Error retrieves any error that occurred during the building and execution of the pipeline.
func (s *Seq) Error() error {
	return s.err
}

func (s *Seq) seed(kind StepKind, sf func(ctx context.Context, emit func(any)) error, opts *stepOpts) {
	s.node(kind, func(ctx context.Context, _ any, emit func(any)) error {
		return sf(ctx, emit)
	}, opts)
}

func (s *Seq) transit(kind StepKind, tf func(ctx context.Context, in any, emit func(any)) error, opts *stepOpts) {
	s.node(kind, func(ctx context.Context, in any, emit func(any)) error {
		return tf(ctx, in, emit)
	}, opts)
}

func (s *Seq) terminal(kind StepKind, tf func(ctx context.Context, in any) error, opts *stepOpts) {
	s.node(kind, func(ctx context.Context, in any, _ func(any)) error {
		return tf(ctx, in)
	}, opts)
}

func (s *Seq) node(kind StepKind, nf func(context.Context, any, func(any)) error, opts *stepOpts) {
	if opts.concurrency < 1 {
		opts.concurrency = 1
	}
	if len(opts.key) == 0 {
		opts.key = fmt.Sprintf("%s-%s", s.keygen(), kind)
	}

	var steps []*Step

	for i := range opts.concurrency {
		replicaKey := opts.key
		if opts.concurrency > 1 {
			replicaKey = fmt.Sprintf("%s-r%d", opts.key, i+1)
		}
		opt := []glow.NodeOpt{
			glow.Key(replicaKey),
			glow.EmitFunc(nf),
		}
		if opts.distributor {
			opt = append(opt, glow.Distributor())
		}
		nodeID, err := s.net.AddNode(opt...)
		s.appendError(err)
		if err == nil {
			step := &Step{
				id:   nodeID,
				kind: kind,
			}
			steps = append(steps, step)
			s.link(step)
		}
	}

	s.preSteps = steps
}

func (s *Seq) link(cur *Step) {
	for _, pre := range s.preSteps {
		if pre != nil {
			if slices.Contains(seedKinds, cur.kind) {
				s.appendError(fmt.Errorf("can not add %s step after %s step", cur.kind, pre.kind))
				return
			}
			if slices.Contains(terminalKinds, pre.kind) {
				s.appendError(fmt.Errorf("can not add %s step after %s step", cur.kind, pre.kind))
				return
			}
			s.appendError(s.net.AddLink(pre.id, cur.id))
		}
	}
}

func (s *Seq) appendError(err error) {
	if err != nil {
		if s.err != nil {
			s.err = fmt.Errorf("%w %w", s.err, err)
		} else {
			s.err = err
		}
	}
}
