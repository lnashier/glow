package flow

import (
	"context"
	"sync"
	"sync/atomic"
)

type StepKind int

func (k StepKind) String() string {
	switch k {
	case ReadStep:
		return "read"
	case MapStep:
		return "map"
	case FilterStep:
		return "filter"
	case CaptureStep:
		return "capture"
	case CollectStep:
		return "collect"
	case CountStep:
		return "count"
	case PeekStep:
		return "peek"
	case CombineStep:
		return "combine"
	default:
		return "unknown"
	}
}

const (
	ReadStep StepKind = iota
	MapStep
	FilterStep
	CaptureStep
	CollectStep
	CountStep
	PeekStep
	CombineStep
)

var linearKinds = []StepKind{
	PeekStep,
	CombineStep,
}

type Step struct {
	id   string
	kind StepKind
}

type StepOpt func(*stepOpts)

type stepOpts struct {
	kind        StepKind
	key         string
	sf          func(context.Context, any, func(any)) error
	replicas    int
	distributor bool
	connections []string
	callback    func()
}

func (o *stepOpts) apply(opt ...StepOpt) *stepOpts {
	for _, op := range opt {
		op(o)
	}
	return o
}

// StepKey sets unique key for the Step.
func StepKey(v string) StepOpt {
	return func(o *stepOpts) {
		o.key = v
	}
}

// Distributor enables a Step to distribute work among next Step, and it's replicas.
// See
//   - Replicas
func Distributor() StepOpt {
	return func(o *stepOpts) {
		o.distributor = true
	}
}

// Replicas sets the number of replicas for the Step, determining how many instances
// of the Step will run concurrently. Depending on whether the preceding Step is in distributing
// or broadcasting mode, these replicas will either operate in a synchronized manner,
// collaborating on the same stream of data, or function independently, each handling the full stream.
func Replicas(v int) StepOpt {
	return func(o *stepOpts) {
		if v > 0 {
			o.replicas = v
		}
	}
}

// Connection sets up a connection between a Step and the Steps identified by the provided key(s).
// The provided keys represent upstream steps, enabling data to flow from these Steps to the current Step.
// Upstream steps can either distribute or broadcast data.
func Connection(key ...string) StepOpt {
	return func(o *stepOpts) {
		o.connections = append(o.connections, key...)
	}
}

// Read retrieves data from a specified source using a provided reader function.
// The reader function is called with a context and an emit function, responsible for
// reading data and emitting it. The emitted data can be of any type.
// Usually, this is the first step, feeding data for processing on to subsequent steps.
func Read(rf func(context.Context, func(any)) error) StepOpt {
	return func(o *stepOpts) {
		o.kind = ReadStep
		o.sf = func(ctx context.Context, _ any, emit func(any)) error {
			return rf(ctx, emit)
		}
	}
}

// Map applies a mapping function to each element in the input data stream.
// The mapper function is invoked with a context, an input element, and an emit function.
// It processes each input element and emits zero or more transformed data points using the 'emit' function.
// The emitted data can be of any type.
// Typically, this step is an intermediate step enabling data transformation operations.
func Map(mf func(ctx context.Context, in any, emit func(any)) error) StepOpt {
	return func(o *stepOpts) {
		o.kind = MapStep
		o.sf = mf
	}
}

// Peek allows observing the data stream without modifying it, typically
// for debugging, logging, or monitoring purposes.
func Peek(pf func(in any)) StepOpt {
	return func(o *stepOpts) {
		o.kind = PeekStep
		o.sf = func(_ context.Context, in any, emit func(any)) error {
			pf(in)
			emit(in)
			return nil
		}
	}
}

// Combine merges the elements of multiple streams into a single stream, concatenating
// all the data points from the individual streams in the order they arrive.
// This enforces the flow to become linear, ensuring that each data point is
// processed sequentially.
// The combined stream can then be broadcast or distributed.
func Combine() StepOpt {
	return func(o *stepOpts) {
		o.kind = CombineStep
		o.sf = func(_ context.Context, in any, emit func(any)) error {
			emit(in)
			return nil
		}
	}
}

// Filter applies a filtering function to each element in the input data stream.
// The filtering function is invoked with an input element and returns a boolean
// indicating whether the element should be retained or not.
// If the filtering function returns true, the element is passed through to the output stream;
// otherwise, it is discarded.
// This step serves as an intermediate step facilitating data filtering operations.
func Filter(ff func(in any) bool) StepOpt {
	return func(o *stepOpts) {
		o.kind = FilterStep
		o.sf = func(ctx context.Context, in any, emit func(any)) error {
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
		}
	}
}

// Capture captures each element in the input data stream and feeds it to a capturing function.
// The capturing function receives a context and captured data point.
// Being a terminal step in the pipeline, it does not emit data.
func Capture(cf func(ctx context.Context, in any) error) StepOpt {
	return func(o *stepOpts) {
		o.kind = CaptureStep
		o.sf = func(ctx context.Context, in any, _ func(any)) error {
			return cf(ctx, in)
		}
	}
}

// Collect aggregates all elements in the input data stream and provides the collection to the provided callback.
// It accepts the Compare function to allow sorting the collected data points as they arrive.
// Compare function must return
//
//	< 0 if a is less than b,
//	= 0 if a equals b,
//	> 0 if a is greater than b.
//
// As a terminal step in the pipeline, it does not emit data, marking the end of the data processing flow.
func Collect(cb func([]any), compare func(a any, b any) int) StepOpt {
	return func(o *stepOpts) {
		o.kind = CollectStep

		mu := &sync.Mutex{}
		var tokens []any
		o.callback = func() {
			cb(tokens)
		}

		o.sf = func(ctx context.Context, in any, _ func(any)) error {
			mu.Lock()
			defer mu.Unlock()
			if compare != nil {
				for i, token := range tokens {
					comp := compare(token, in)
					if comp > 0 {
						tokens = append(tokens[:i+1], tokens[i:]...)
						tokens[i] = in
						return nil
					}
				}
			}
			tokens = append(tokens, in)
			return nil
		}

	}
}

// Count keeps track of the number of elements in the input data stream.
// Being a terminal step in the pipeline, it does not emit data.
func Count(cb func(num int)) StepOpt {
	return func(o *stepOpts) {
		o.kind = CountStep

		var count atomic.Int64
		o.callback = func() {
			cb(int(count.Load()))
		}

		o.sf = func(ctx context.Context, in any, _ func(any)) error {
			count.Add(1)
			return nil
		}
	}
}
