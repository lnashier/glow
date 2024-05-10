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
}

var terminalKinds = []StepKind{
	CaptureStep,
	CollectStep,
	CountStep,
}
