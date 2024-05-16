package flow

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

var seedKinds = []StepKind{
	ReadStep,
}

var transitKinds = []StepKind{
	MapStep,
	FilterStep,
	PeekStep,
	CombineStep,
}

var terminalKinds = []StepKind{
	CaptureStep,
	CollectStep,
	CountStep,
}

var linearKinds = []StepKind{
	PeekStep,
	CollectStep,
	CountStep,
}

type Step struct {
	id   string
	kind StepKind
}

type StepOpt func(*stepOpts)

type stepOpts struct {
	key         string
	compare     func(any, any) int
	concurrency int
	distributor bool
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

// Compare function must return
//
//	< 0 if a is less than b,
//	= 0 if a equals b,
//	> 0 if a is greater than b.
func Compare(f func(a any, b any) int) StepOpt {
	return func(o *stepOpts) {
		o.compare = f
	}
}

// Distributor enables a Step to distribute work among next Step, and it's replicas.
// See
//   - Concurrency
func Distributor() StepOpt {
	return func(o *stepOpts) {
		o.distributor = true
	}
}

// Concurrency sets the number of replicas for the Step, determining how many instances
// of the Step will run concurrently. Depending on whether the preceding Step is in distributing
// or broadcasting mode, these replicas will either operate in a synchronized manner,
// collaborating on the same stream of data, or function independently, each handling the full stream.
func Concurrency(v int) StepOpt {
	return func(o *stepOpts) {
		if v > 0 {
			o.concurrency = v
		}
	}
}
