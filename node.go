package glow

import (
	"context"
	"errors"
	"golang.org/x/sync/errgroup"
	"slices"
	"strings"
)

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

func (n *Network) nodeUp(ctx context.Context, key string) error {
	n.log("Node(%s) coming up", key)
	defer n.log("Node(%s) shut down", key)

	node, err := n.Node(key)
	if err != nil {
		return err
	}

	ingress := slices.DeleteFunc(n.Ingress(node.key), func(l *Link) bool {
		return l.paused || l.deleted
	})
	egress := slices.DeleteFunc(n.Egress(node.key), func(l *Link) bool {
		return l.paused || l.deleted
	})

	n.log("Node(%s) ingress(%v) egress(%v)", node.key, ingress, egress)

	if len(ingress) == 0 && len(egress) == 0 {
		if n.ignoreIsolatedNodes {
			return nil
		}
		return ErrIsolatedNodeFound
	}

	egressYs := ""
	if len(egress) > 0 && node.distributor {
		for _, egressLink := range egress {
			egressYs += egressLink.y + ","
		}
		egressYs = strings.TrimSuffix(egressYs, ",")
	}

	switch {
	case len(ingress) == 0 && len(egress) > 0:
		n.log("Seed(%s) running", node.key)

		for {
			select {
			case <-ctx.Done():
				n.log("Seed(%s) net-ctx done", node.key)
				return nil
			default:
				nodeData, nodeErr := node.f(ctx, nil)
				if nodeErr != nil {
					if errors.Is(nodeErr, ErrSeedingDone) || errors.Is(nodeErr, ErrNodeGoingAway) {
						n.log("Seed(%s) %v", node.key, nodeErr)
						return nil
					}
					n.log("Seed(%s) Err: %v", node.key, nodeErr)
					return nodeErr
				}

				if node.distributor {
					// Get any egress link, they all share same channel
					egressLink := egress[0]
					n.log("Seed(%s/%s) Distributing Data(%v) To Nodes(%s)", node.key, egressLink.x, nodeData, egressYs)
					select {
					case <-ctx.Done():
						n.log("Seed(%s/%s) net-ctx done while distributing Data(%v) To Nodes(%s)", node.key, egressLink.x, nodeData, egressYs)
						return nil
					case egressLink.ch <- nodeData:
						n.log("Seed(%s/%s) Distributed Data(%v) To Nodes(%s)", node.key, egressLink.x, nodeData, egressYs)
					}
				} else {
					for _, egressLink := range egress {
						n.log("Seed(%s/%s) Sending Data(%v) To Node(%s)", node.key, egressLink.x, nodeData, egressLink.y)
						select {
						case <-ctx.Done():
							n.log("Seed(%s/%s) net-ctx done while sending Data(%v) To Node(%s)", node.key, egressLink.x, nodeData, egressLink.y)
							return nil
						case egressLink.ch <- nodeData:
							n.log("Seed(%s/%s) Sent Data(%v) To Node(%s)", node.key, egressLink.x, nodeData, egressLink.y)
						}
					}
				}
			}
		}
	case len(ingress) > 0 && len(egress) > 0:
		n.log("Node(%s) running", node.key)

		nodewg, nodectx := errgroup.WithContext(ctx)

		for _, ingressLink := range ingress {
			nodewg.Go(func() error {
				for {
					select {
					case <-nodectx.Done():
						n.log("Node(%s/%s) node-ctx done for Node(%s)", node.key, ingressLink.y, ingressLink.x)
						return nil
					case inData := <-ingressLink.ch:
						ingressLink.tally++
						n.log("Node(%s/%s) Received Data(%v) From(%s)", node.key, ingressLink.y, inData, ingressLink.x)

						nodeData, nodeErr := node.f(nodectx, inData)
						if nodeErr != nil {
							if errors.Is(nodeErr, ErrNodeGoingAway) {
								n.log("Node(%s/%s) %v for Node(%s)", node.key, ingressLink.y, nodeErr, ingressLink.x)
								return nil
							}
							n.log("Node(%s/%s) Err: %v for Node(%s)", node.key, ingressLink.y, nodeErr, ingressLink.x)
							return nodeErr
						}

						if node.distributor {
							// Get any egress link, they all share same channel
							egressLink := egress[0]
							n.log("Node(%s/%s) Distributing Data(%v) Of(%s) To Nodes(%s)", node.key, egressLink.x, nodeData, ingressLink.x, egressYs)
							select {
							case <-nodectx.Done():
								n.log("Node(%s/%s) node-ctx done while distributing Data(%v) Of(%s) To Nodes(%s)", node.key, egressLink.x, nodeData, ingressLink.x, egressYs)
								return nil
							case egressLink.ch <- nodeData:
								n.log("Node(%s/%s) Distributed Data(%v) Of(%s) To Nodes(%s)", node.key, egressLink.x, nodeData, ingressLink.y, egressYs)
							}
						} else {
							for _, egressLink := range egress {
								n.log("Node(%s/%s) Sending Data(%v) Of(%s) To Node(%s)", node.key, egressLink.x, nodeData, ingressLink.x, egressLink.y)
								select {
								case <-nodectx.Done():
									n.log("Node(%s/%s) node-ctx done while sending Data(%v) Of(%s) To Node(%s)", node.key, egressLink.x, nodeData, ingressLink.x, egressLink.y)
									return nil
								case egressLink.ch <- nodeData:
									n.log("Node(%s/%s) Sent Data(%v) Of(%s) To Node(%s)", node.key, egressLink.x, nodeData, ingressLink.y, egressLink.y)
								}
							}
						}
					}
				}
			})
		}

		if err := nodewg.Wait(); err != nil {
			return err
		}
	case len(ingress) > 0 && len(egress) == 0:
		n.log("Terminus(%s) running", node.key)

		nodewg, nodectx := errgroup.WithContext(ctx)

		for _, ingressLink := range ingress {
			nodewg.Go(func() error {
				for {
					select {
					case <-nodectx.Done():
						n.log("Terminus(%s/%s) node-ctx done for Node(%s)", node.key, ingressLink.y, ingressLink.x)
						return nil
					case inData := <-ingressLink.ch:
						ingressLink.tally++
						n.log("Terminus(%s/%s) Received Data(%v) From(%s)", node.key, ingressLink.y, inData, ingressLink.x)

						nodeData, nodeErr := node.f(nodectx, inData)
						if nodeErr != nil {
							if errors.Is(nodeErr, ErrNodeGoingAway) {
								n.log("Terminus(%s/%s) %v for Node(%s)", node.key, ingressLink.y, nodeErr, ingressLink.x)
								return nil
							}
							n.log("Terminus(%s/%s) Err: %v for Node(%s)", node.key, ingressLink.y, nodeErr, ingressLink.x)
							return nodeErr
						}
						n.log("Terminus(%s/%s) Data(%v) for Node(%s)", node.key, ingressLink.y, nodeData, ingressLink.x)
					}
				}
			})
		}

		if err := nodewg.Wait(); err != nil {
			return err
		}
	}

	return nil
}
