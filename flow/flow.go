package flow

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	"github.com/lnashier/glow/help"
	"github.com/lnashier/goarc"
	"slices"
)

type Seq struct {
	net       *glow.Network
	keygen    func() string
	preStep   *Step
	err       error
	callbacks []func()
	branching bool
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
func (s *Seq) Read(rf func(ctx context.Context, emit func(any)) error) *Seq {
	return s.seed(ReadStep, rf)
}

// Map applies a mapping function to each element in the input data stream.
// The mapper function is invoked with a context, an input element, and an emit function.
// It processes each input element and emits zero or more transformed data points using the 'emit' function.
// The emitted data can be of any type.
// Typically, this step is an intermediate step enabling data transformation operations.
func (s *Seq) Map(mf func(ctx context.Context, in any, emit func(any)) error) *Seq {
	return s.transit(MapStep, mf)
}

// Peek allows observing the data stream without modifying it, typically
// for debugging, logging, or monitoring purposes.
func (s *Seq) Peek(pf func(in any)) *Seq {
	return s.transit(PeekStep, func(ctx context.Context, in any, emit func(any)) error {
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
func (s *Seq) Filter(ff func(in any) bool) *Seq {
	return s.transit(FilterStep, func(ctx context.Context, in any, emit func(any)) error {
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
func (s *Seq) Capture(cf func(ctx context.Context, in any) error) *Seq {
	return s.terminal(CaptureStep, cf)
}

// Collect aggregates all elements in the input data stream and provides the collection to provided callback.
// It accepts the following options:
//   - Compare option allows sorting the collection before passing it to the callback.
func (s *Seq) Collect(cb func([]any), opt ...Opt) *Seq {
	nodeOpts := &opts{}
	nodeOpts.apply(opt...)

	var tokens []any

	s.terminal(CollectStep, func(ctx context.Context, in any) error {
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

	s.callbacks = append(s.callbacks, func() {
		cb(tokens)
	})

	return s
}

// Count counts the number of elements in the input data stream.
func (s *Seq) Count(cb func(num int)) *Seq {
	var tokens []any
	s.terminal(CountStep, func(ctx context.Context, in any) error {
		tokens = append(tokens, in)
		return nil
	})
	s.callbacks = append(s.callbacks, func() {
		cb(len(tokens))
	})
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

// Error retrieves any error that occurred during the building
// and execution of the pipeline.
func (s *Seq) Error() error {
	return s.err
}

// Draw generates a Graphviz visualization of the Flow.
func (s *Seq) Draw(name string) *Seq {
	s.appendError(help.Draw(s.net, name))
	return s
}

func (s *Seq) seed(kind StepKind, sf func(ctx context.Context, emit func(any)) error) *Seq {
	nodeID, err := s.net.AddNode(
		glow.EmitterFunc(func(ctx context.Context, _ any, emit func(any)) error {
			return sf(ctx, emit)
		}),
		glow.KeyFunc(s.keygen),
	)
	s.appendError(err)
	s.link(&Step{
		id:   nodeID,
		kind: kind,
	})
	return s
}

func (s *Seq) transit(kind StepKind, tf func(ctx context.Context, in any, emit func(any)) error) *Seq {
	nodeID, err := s.net.AddNode(
		glow.EmitterFunc(func(ctx context.Context, in any, emit func(any)) error {
			return tf(ctx, in, emit)
		}),
		glow.KeyFunc(s.keygen),
	)
	s.appendError(err)
	s.link(&Step{
		id:   nodeID,
		kind: kind,
	})
	return s
}

func (s *Seq) terminal(kind StepKind, tf func(ctx context.Context, in any) error) *Seq {
	nodeID, err := s.net.AddNode(
		glow.NodeFunc(func(ctx context.Context, in any) (any, error) {
			return nil, tf(ctx, in)
		}),
		glow.KeyFunc(s.keygen),
	)
	s.appendError(err)
	s.link(&Step{
		id:   nodeID,
		kind: kind,
	})
	return s
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

func (s *Seq) link(step *Step) {
	if s.preStep != nil {
		if slices.Contains(seedKinds, step.kind) {
			s.appendError(fmt.Errorf("can not add %s step after %s step", step.kind, s.preStep.kind))
			return
		}
		if slices.Contains(terminalKinds, s.preStep.kind) {
			s.appendError(fmt.Errorf("can not add %s step after %s step", step.kind, s.preStep.kind))
			return
		}
		s.appendError(s.net.AddLink(s.preStep.id, step.id))
	}
	s.preStep = step
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
