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

type Step struct {
	id   string
	kind StepKind
}

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
