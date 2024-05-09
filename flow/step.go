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
	default:
		return "unknown"
	}
}

const (
	ReadStep StepKind = iota
	MapStep
	FilterStep
	CaptureStep
)

type Step struct {
	id   string
	kind StepKind
}
