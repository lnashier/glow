package glow

import "context"

// Node represents a node within the [Network].
// If a Node has only egress Links, it's a Seed node.
// If a Node has only ingress Links, it's a Terminus node.
// By default, a Node operates in broadcaster mode unless the distributor flag is set.
type Node struct {
	key         string
	f           NodeFunc
	distributor bool
}

// NodeFunc is the function responsible for processing incoming data on the Node.
type NodeFunc func(context.Context, any) (any, error)

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

// AddNode adds a new Node in the network.
// Node key is retrieved from the provided [Key] function if not given.
func (n *Network) AddNode(node NodeFunc, opt ...NodeOpt) (string, error) {
	// check if session is in progress
	n.session.mu.RLock()
	defer n.session.mu.RUnlock()

	n.mu.Lock()
	defer n.mu.Unlock()

	opts := defaultNodeOpts
	opts.apply(opt)

	k := opts.key
	if len(k) == 0 && opts.keyFunc != nil {
		k = opts.keyFunc()
	}
	if len(k) == 0 {
		return k, ErrBadNodeKey
	}

	if _, ok := n.nodes[k]; ok {
		return k, ErrNodeAlreadyExists
	}

	n.nodes[k] = &Node{
		key:         k,
		distributor: opts.distributor,
		f:           node,
	}

	return k, nil
}

// Node returns the node identified by the provided key.
func (n *Network) Node(k string) (*Node, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	node, ok := n.nodes[k]
	if !ok {
		return node, ErrNodeNotFound
	}

	return node, nil
}

// RemoveNode removes a node with provided key.
// A node can't be removed if it is connected/linked to any other node in the network.
func (n *Network) RemoveNode(k string) error {
	// check if session is in progress
	n.session.mu.RLock()
	defer n.session.mu.RUnlock()

	n.mu.Lock()
	defer n.mu.Unlock()

	return n.removeNode(k)
}

func (n *Network) removeNode(k string) error {
	if _, ok := n.nodes[k]; !ok {
		return ErrNodeNotFound
	}
	if len(n.ingress[k]) > 0 {
		return ErrNodeIsConnected
	}
	if len(n.egress[k]) > 0 {
		return ErrNodeIsConnected
	}
	delete(n.ingress, k) // just removing empty map
	delete(n.egress, k)  // just removing empty map
	delete(n.nodes, k)
	return nil
}

// Nodes returns all the nodes as their unique keys in the Network.
// Node should be called to get actual node.
func (n *Network) Nodes() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	keys := make([]string, 0, len(n.nodes))
	for k := range n.nodes {
		keys = append(keys, k)
	}

	return keys
}

// Seeds returns all the nodes that have only egress links.
// Node should be called to get actual node.
func (n *Network) Seeds() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var keys []string

	for k := range n.nodes {
		if len(n.Ingress(k)) == 0 && len(n.Egress(k)) > 0 {
			keys = append(keys, k)
		}
	}

	return keys
}

// Termini returns all the nodes that have only ingress links.
// Node should be called to get actual node.
func (n *Network) Termini() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var keys []string

	for k := range n.nodes {
		if len(n.Ingress(k)) > 0 && len(n.Egress(k)) == 0 {
			keys = append(keys, k)
		}
	}

	return keys
}
