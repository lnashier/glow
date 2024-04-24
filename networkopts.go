package glow

import "time"

type NetworkOpt func(*networkOpts)

type networkOpts struct {
	verbose             bool
	stopGracetime       time.Duration
	ignoreIsolatedNodes bool
}

var defaultNetworkOpts = networkOpts{}

func (s *networkOpts) apply(opts []NetworkOpt) {
	for _, o := range opts {
		o(s)
	}
}

func Verbose() NetworkOpt {
	return func(s *networkOpts) {
		s.verbose = true
	}
}

func IgnoreIsolatedNodes() NetworkOpt {
	return func(s *networkOpts) {
		s.ignoreIsolatedNodes = true
	}
}

func StopGracetime(t time.Duration) NetworkOpt {
	return func(s *networkOpts) {
		s.stopGracetime = t
	}
}
