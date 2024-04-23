package glow

type NodeOpt func(*nodeOpts)

type nodeOpts struct {
	keyFunc func() string
	key     string
}

var defaultNodeOpts = nodeOpts{}

func (s *nodeOpts) apply(opts []NodeOpt) {
	for _, o := range opts {
		o(s)
	}
}

// KeyFunc sets function to generate unique keys for the Node.
func KeyFunc(k func() string) NodeOpt {
	return func(s *nodeOpts) {
		s.keyFunc = k
	}
}

// Key sets the key for the Node.
func Key(k string) NodeOpt {
	return func(s *nodeOpts) {
		s.key = k
	}
}
