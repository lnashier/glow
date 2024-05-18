package flow

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	"time"
)

type Seq struct {
	plan    *Plan
	keygen  func() string
	preStep string
}

func Sequential(opt ...glow.NetworkOpt) *Seq {
	opt = append(opt, glow.PreventCycles())
	return &Seq{
		plan:   New(opt...),
		keygen: Keygen("step"),
	}
}

func (s *Seq) Read(rf func(ctx context.Context, emit func(any)) error, opt ...StepOpt) *Seq {
	s.step(ReadStep, append(opt, Read(rf))...)
	return s
}

func (s *Seq) Map(mf func(ctx context.Context, in any, emit func(any)) error, opt ...StepOpt) *Seq {
	s.step(MapStep, append(opt, Map(mf))...)
	return s
}

func (s *Seq) Peek(pf func(in any), opt ...StepOpt) *Seq {
	s.step(PeekStep, append(opt, Peek(pf))...)
	return s
}

func (s *Seq) Combine(opt ...StepOpt) *Seq {
	s.step(CombineStep, append(opt, Combine())...)
	return s
}

func (s *Seq) Filter(ff func(in any) bool, opt ...StepOpt) *Seq {
	s.step(FilterStep, append(opt, Filter(ff))...)
	return s
}

func (s *Seq) Capture(cf func(ctx context.Context, in any) error, opt ...StepOpt) *Seq {
	s.step(CaptureStep, append(opt, Capture(cf))...)
	return s
}

func (s *Seq) Collect(cb func([]any), compare func(a any, b any) int, opt ...StepOpt) *Seq {
	s.step(CollectStep, append(opt, Collect(cb, compare))...)
	return s
}

func (s *Seq) Count(cb func(num int), opt ...StepOpt) *Seq {
	s.step(CountStep, append(opt, Count(cb))...)
	return s
}

func (s *Seq) Run(ctx context.Context) *Seq {
	s.plan.Run(ctx)
	return s
}

func (s *Seq) Stop() *Seq {
	s.plan.Stop()
	return s
}

func (s *Seq) Draw(name string) *Seq {
	s.plan.Draw(name)
	return s
}

func (s *Seq) Uptime(uf func(d time.Duration)) *Seq {
	s.plan.Uptime(uf)
	return s
}

func (s *Seq) Error() error {
	return s.plan.err
}

func (s *Seq) step(kind StepKind, opt ...StepOpt) *Seq {
	opts := (&stepOpts{}).apply(opt...)
	key := opts.key
	// Even if key is supplied, call keygen() to keep node numbers relevant
	keyPart := s.keygen()
	if len(key) == 0 {
		key = fmt.Sprintf("%s-%s", keyPart, kind)
	}
	s.plan.Step(append(opt, StepKey(key), Connection(s.preStep))...)
	s.preStep = key
	return s
}
