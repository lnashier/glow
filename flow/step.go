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

type Step struct {
	id   string
	kind StepKind
}

type StepOpt func(*stepOpts)

type stepOpts struct {
	keyToken    string
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

func KeyToken(v string) StepOpt {
	return func(o *stepOpts) {
		o.keyToken = v
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

func Concurrency(v int) StepOpt {
	return func(o *stepOpts) {
		if v > 0 {
			o.concurrency = v
		}
	}
}

func Distributor() StepOpt {
	return func(o *stepOpts) {
		o.distributor = true
	}
}
