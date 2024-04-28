package glow

import "context"

// Node defines a node in the [Network].
// Node is said to be Seed node if it has only egress Links.
// Node is said to be Terminus node if it has only ingress Links.
type Node struct {
	key         string
	f           NodeFunc
	distributor bool
}

// NodeFunc is the function responsible for processing incoming data on the Node.
type NodeFunc func(context.Context, []byte) ([]byte, error)

type NodeOpt func(*nodeOpts)

type nodeOpts struct {
	keyFunc     func() string
	key         string
	distributor bool
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

func Distributor() NodeOpt {
	return func(s *nodeOpts) {
		s.distributor = true
	}
}
