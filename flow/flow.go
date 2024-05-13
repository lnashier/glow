package flow

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	"github.com/lnashier/glow/help"
	"github.com/lnashier/goarc"
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
	return &Seq{
		net:    glow.New(opt...),
		keygen: Keygen("node"),
	}
}

// Read retrieves data from a specified source using a provided reader function.
// The reader function is called with a context and an emit function, responsible for
// reading data and emitting it. The emitted data can be of any type.
// Usually, this is the first step, feeding data for processing on to subsequent steps.
func (s *Seq) Read(rf func(ctx context.Context, emit func(any)) error, opt ...Opt) *Seq {
	stepOpts := &opts{}
	stepOpts.apply(opt...)
	step, err := s.seed(ReadStep, rf, stepOpts)
	s.appendError(err)
	s.link(step)
	s.preSteps = []*Step{step}
	return s
}

// Map applies a mapping function to each element in the input data stream.
// The mapper function is invoked with a context, an input element, and an emit function.
// It processes each input element and emits zero or more transformed data points using the 'emit' function.
// The emitted data can be of any type.
// Typically, this step is an intermediate step enabling data transformation operations.
func (s *Seq) Map(mf func(ctx context.Context, in any, emit func(any)) error, opt ...Opt) *Seq {
	stepOpts := &opts{
		concurrency: 1,
	}
	stepOpts.apply(opt...)

	var steps []*Step

	for range stepOpts.concurrency {
		step, err := s.transit(MapStep, mf, stepOpts)
		s.appendError(err)
		steps = append(steps, step)
		s.link(step)
	}

	s.preSteps = steps

	return s
}

// Peek allows observing the data stream without modifying it, typically
// for debugging, logging, or monitoring purposes.
func (s *Seq) Peek(pf func(in any)) *Seq {
	step, err := s.transit(PeekStep, func(ctx context.Context, in any, emit func(any)) error {
		pf(in)
		emit(in)
		return nil
	}, &opts{})
	s.appendError(err)
	s.link(step)
	s.preSteps = []*Step{step}
	return s
}

// Combine combines the elements of streams into a single stream, concatenating
// all the data points from the individual streams in the order they arrive.
func (s *Seq) Combine() *Seq {
	step, err := s.transit(CombineStep, func(ctx context.Context, in any, emit func(any)) error {
		emit(in)
		return nil
	}, &opts{})
	s.appendError(err)
	s.link(step)
	s.preSteps = []*Step{step}
	return s
}

// Filter applies a filtering function to each element in the input data stream.
// The filtering function is invoked with an input element and returns a boolean
// indicating whether the element should be retained or not.
// If the filtering function returns true, the element is passed through to the output stream;
// otherwise, it is discarded.
// This step serves as an intermediate step facilitating data filtering operations.
func (s *Seq) Filter(ff func(in any) bool, opt ...Opt) *Seq {
	stepOpts := &opts{
		concurrency: 1,
	}
	stepOpts.apply(opt...)

	var steps []*Step

	for range stepOpts.concurrency {
		step, err := s.transit(FilterStep, func(ctx context.Context, in any, emit func(any)) error {
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
		}, stepOpts)
		s.appendError(err)
		steps = append(steps, step)
		s.link(step)
	}

	s.preSteps = steps

	return s
}

// Capture captures each element in the input data stream and feeds it to a capturing function.
// The capturing function receives a context and captured data point.
// Being a terminal step in the pipeline, it does not emit data.
func (s *Seq) Capture(cf func(ctx context.Context, in any) error) *Seq {
	step, err := s.terminal(CaptureStep, cf, &opts{})
	s.appendError(err)
	s.link(step)
	s.preSteps = []*Step{step}
	return s
}

// Collect aggregates all elements in the input data stream and provides the collection to provided callback.
// It accepts the following options:
//   - Compare option allows sorting the collection before passing it to the callback.
func (s *Seq) Collect(cb func([]any), opt ...Opt) *Seq {
	stepOpts := &opts{
		compare: func(a any, b any) int {
			return 0
		},
	}
	stepOpts.apply(opt...)

	var tokens []any
	s.callbacks = append(s.callbacks, func() {
		cb(tokens)
	})

	step, err := s.terminal(CollectStep, func(ctx context.Context, in any) error {
		for i, token := range tokens {
			comp := stepOpts.compare(token, in)
			if comp > 0 {
				tokens = append(tokens[:i+1], tokens[i:]...)
				tokens[i] = in
				return nil
			}
		}
		tokens = append(tokens, in)
		return nil
	}, stepOpts)
	s.appendError(err)
	s.link(step)
	s.preSteps = []*Step{step}

	return s
}

// Count counts the number of elements in the input data stream.
func (s *Seq) Count(cb func(num int)) *Seq {
	var tokens []any
	s.callbacks = append(s.callbacks, func() {
		cb(len(tokens))
	})

	step, err := s.terminal(CountStep, func(ctx context.Context, in any) error {
		tokens = append(tokens, in)
		return nil
	}, &opts{})
	s.appendError(err)
	s.link(step)
	s.preSteps = []*Step{step}

	return s
}

// Run initiates the processing of data through the pipeline, starting from the
// initial input source and sequentially applying each operation
// defined in the pipeline until reaching the terminal step.
func (s *Seq) Run() *Seq {
	if s.err != nil {
		return s
	}
	goarc.Up(
		s.net,
		goarc.OnStart(func(err error) {
			s.appendError(err)
		}),
		goarc.OnStop(func(err error) {
			s.appendError(err)
		}),
	)
	for _, callback := range s.callbacks {
		callback()
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

// Error retrieves any error that occurred during the building
// and execution of the pipeline.
func (s *Seq) Error() error {
	return s.err
}

func (s *Seq) seed(kind StepKind, sf func(ctx context.Context, emit func(any)) error, opts *opts) (*Step, error) {
	nodeOpts := []glow.NodeOpt{
		glow.EmitFunc(func(ctx context.Context, _ any, emit func(any)) error {
			return sf(ctx, emit)
		}),
		glow.Key(fmt.Sprintf("%s-%s", s.keygen(), kind)),
	}
	if opts.distributor {
		nodeOpts = append(nodeOpts, glow.Distributor())
	}
	return s.node(kind, nodeOpts...)
}

func (s *Seq) transit(kind StepKind, tf func(ctx context.Context, in any, emit func(any)) error, opts *opts) (*Step, error) {
	nodeOpts := []glow.NodeOpt{
		glow.EmitFunc(func(ctx context.Context, in any, emit func(any)) error {
			return tf(ctx, in, emit)
		}),
		glow.Key(fmt.Sprintf("%s-%s", s.keygen(), kind)),
	}
	if opts.distributor {
		nodeOpts = append(nodeOpts, glow.Distributor())
	}
	return s.node(kind, nodeOpts...)
}

func (s *Seq) terminal(kind StepKind, tf func(ctx context.Context, in any) error, opts *opts) (*Step, error) {
	nodeOpts := []glow.NodeOpt{
		glow.NodeFunc(func(ctx context.Context, in any) (any, error) {
			return nil, tf(ctx, in)
		}),
		glow.Key(fmt.Sprintf("%s-%s", s.keygen(), kind)),
	}
	if opts.distributor {
		nodeOpts = append(nodeOpts, glow.Distributor())
	}
	return s.node(kind, nodeOpts...)
}

func (s *Seq) node(kind StepKind, opt ...glow.NodeOpt) (*Step, error) {
	nodeID, err := s.net.AddNode(opt...)
	return &Step{
		id:   nodeID,
		kind: kind,
	}, err
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

type Opt func(*opts)

type opts struct {
	compare     func(any, any) int
	concurrency int
	distributor bool
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

func Concurrency(v int) Opt {
	return func(o *opts) {
		if v > 0 {
			o.concurrency = v
		}
	}
}

func Distributor() Opt {
	return func(o *opts) {
		o.distributor = true
	}
}
