package glow

type LinkOpt func(*linkOpts)

type linkOpts struct {
	size int
}

var defaultLinkOpts = linkOpts{}

func (s *linkOpts) apply(opts []LinkOpt) {
	for _, o := range opts {
		o(s)
	}
}

// Size sets the link bandwidth for the Link.
func Size(k int) LinkOpt {
	return func(s *linkOpts) {
		s.size = k
	}
}
