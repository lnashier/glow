package glow

import (
	"context"
	"errors"
	"golang.org/x/sync/errgroup"
	"slices"
	"strings"
)

// Node represents a node within the [Network].
//
// Node types:
//   - With no Links, Node is considered an isolated-node.
//   - With only egress Links, Node is considered a seed-node.
//   - With only ingress Links, Node is considered a terminus-node.
//   - With both egress and ingress Links, Node is considered a transit-node.
//
// Node operating modes:
//   - By default, a Node operates in broadcaster mode unless the distributor flag is set.
//     In broadcaster mode, Node broadcasts all incoming data to all outgoing links.
//     When the distributor flag is enabled, a Node distributes incoming data among its outgoing links.
//     Distributor mode is not functional for isolated and terminus nodes.
//   - By default, a Node operates in "push-pull" mode, where the Network pushes data to NodeFunc and waits for NodeFunc to return.
//     This behavior can be changed to "push-push" by setting the emitter setting for the Node.
//     In emitter mode, the Network pushes data to NodeFunc, and the Node emits data back to the Network through the supplied channel.
//     Emitter mode is not functional for isolated and terminus nodes.
type Node struct {
	key         string
	f           func(context.Context, any) (any, error)
	ef          func(context.Context, any, func(any)) error
	distributor bool
}

type NodeOpt func(*Node)

func (n *Node) apply(opt ...NodeOpt) {
	for _, o := range opt {
		o(n)
	}
}

// KeyFunc sets function to generate unique keys for the Node.
func KeyFunc(f func() string) NodeOpt {
	return func(n *Node) {
		n.key = f()
	}
}

// Key sets the key for the Node.
func Key(k string) NodeOpt {
	return func(n *Node) {
		n.key = k
	}
}

func Distributor() NodeOpt {
	return func(n *Node) {
		n.distributor = true
	}
}

// NodeFunc is the function responsible for processing incoming data on the Node.
func NodeFunc(f func(ctx context.Context, data any) (any, error)) NodeOpt {
	return func(n *Node) {
		n.f = f
	}
}

// EmitterFunc is similar to NodeFunc, but it additionally provides a callback where data can be emitted.
func EmitterFunc(f func(ctx context.Context, data any, emit func(any)) error) NodeOpt {
	return func(n *Node) {
		n.ef = f
	}
}

// AddNode adds a new Node in the network.
// Node key is retrieved from the provided [Key] function if not given.
func (n *Network) AddNode(opt ...NodeOpt) (string, error) {
	// check if session is in progress
	n.session.mu.RLock()
	defer n.session.mu.RUnlock()

	n.mu.Lock()
	defer n.mu.Unlock()

	node := &Node{}
	node.apply(opt...)

	if len(node.key) == 0 {
		return node.key, ErrBadNodeKey
	}

	if _, ok := n.nodes[node.key]; ok {
		return node.key, ErrNodeAlreadyExists
	}

	if node.f == nil && node.ef == nil {
		return node.key, ErrNodeFunctionMissing
	}
	if node.f != nil && node.ef != nil {
		return node.key, ErrTooManyNodeFunction
	}

	n.nodes[node.key] = node

	return node.key, nil
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
// A node can't be removed if it is linked to any other node in the Network.
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
// Network.Node should be called to get actual node.
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
// Network.Node should be called to get actual node.
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
// Network.Node should be called to get actual node.
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

func (n *Network) nodeUp(ctx context.Context, node *Node) error {
	n.log("Node(%s) coming up", node.key)
	defer n.log("Node(%s) shut down", node.key)

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

	var egressYs string
	if len(egress) > 0 && node.distributor {
		for _, egressLink := range egress {
			egressYs += egressLink.y + ","
		}
		egressYs = strings.TrimSuffix(egressYs, ",")
	}

	switch {
	case len(ingress) == 0 && len(egress) > 0:
		n.log("Seed Node(%s) is running", node.key)

		if node.ef != nil {
			nodeWg, nodeCtx := errgroup.WithContext(ctx)
			nodeDataCh := make(chan any)

			nodeWg.Go(func() error {
				// When seed-node is in emitter mod, Node emitter function is called once.
				// Seed-node can choose to emit as many data points and exit eventually.
				err := node.ef(nodeCtx, nil, func(nodeData any) {
					select {
					case <-nodeCtx.Done():
					case nodeDataCh <- nodeData:
					}
				})
				close(nodeDataCh)
				return err
			})

			nodeWg.Go(func() error {
				for {
					select {
					case <-nodeCtx.Done():
						n.log("Seed(%s) node-ctx done while reading emitted data", node.key)
						return nil
					case nodeData, ok := <-nodeDataCh:
						if !ok {
							n.log("Seed(%s) Data Channel Closed", node.key)
							return nil
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
			})

			if err := nodeWg.Wait(); err != nil {
				if errors.Is(err, ErrSeedingDone) || errors.Is(err, ErrNodeGoingAway) {
					n.log("Seed(%s) %v", node.key, err)
					return nil
				}
				n.log("Seed(%s) Err: %v", node.key, err)
				return err
			}
		} else {
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
		}
	case len(ingress) > 0 && len(egress) > 0:
		n.log("Transit Node(%s) is running", node.key)

		nodeWg, nodeCtx := errgroup.WithContext(ctx)

		for _, ingressLink := range ingress {
			nodeWg.Go(func() error {
				if node.ef != nil {
					inDataWg, inDataCtx := errgroup.WithContext(nodeCtx)
					nodeDataCh := make(chan any)

					inDataWg.Go(func() error {
						for {
							select {
							case <-inDataCtx.Done():
								n.log("Node(%s/%s) in-node-ctx done for Node(%s)", node.key, ingressLink.y, ingressLink.x)
								return nil
							case inData := <-ingressLink.ch:
								ingressLink.tally++
								n.log("Node(%s/%s) Received Data(%v) From(%s)", node.key, ingressLink.y, inData, ingressLink.x)

								// When transit-node is in emitter mod, Node emitter function is called for every incoming data point.
								// Transit-node can choose to emit as many data points and return control back to get next incoming data point.
								if err := node.ef(inDataCtx, inData, func(nodeData any) {
									select {
									case <-inDataCtx.Done():
									case nodeDataCh <- nodeData:
									}
								}); err != nil {
									return err
								}
							}
						}
					})

					inDataWg.Go(func() error {
						for {
							select {
							case <-inDataCtx.Done():
								n.log("Node(%s/%s) node-ctx done for Node(%s) while reading emitted data", node.key, ingressLink.y, ingressLink.x)
								return nil
							case nodeData, ok := <-nodeDataCh:
								if !ok {
									n.log("Node(%s/%s) Data Channel Closed for Node(%s)", node.key, ingressLink.y, ingressLink.x)
									return nil
								}

								if node.distributor {
									// Get any egress link, they all share same channel
									egressLink := egress[0]
									n.log("Node(%s/%s) Distributing Data(%v) Of(%s) To Nodes(%s)", node.key, egressLink.x, nodeData, ingressLink.x, egressYs)
									select {
									case <-nodeCtx.Done():
										n.log("Node(%s/%s) node-ctx done while distributing Data(%v) Of(%s) To Nodes(%s)", node.key, egressLink.x, nodeData, ingressLink.x, egressYs)
										return nil
									case egressLink.ch <- nodeData:
										n.log("Node(%s/%s) Distributed Data(%v) Of(%s) To Nodes(%s)", node.key, egressLink.x, nodeData, ingressLink.y, egressYs)
									}
								} else {
									for _, egressLink := range egress {
										n.log("Node(%s/%s) Sending Data(%v) Of(%s) To Node(%s)", node.key, egressLink.x, nodeData, ingressLink.x, egressLink.y)
										select {
										case <-nodeCtx.Done():
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

					if err := inDataWg.Wait(); err != nil {
						if errors.Is(err, ErrNodeGoingAway) {
							n.log("Node(%s/%s) %v for Node(%s)", node.key, ingressLink.y, err, ingressLink.x)
							return nil
						}
						n.log("Node(%s/%s) Err: %v for Node(%s)", node.key, ingressLink.y, err, ingressLink.x)
						return err
					}
				} else {
					for {
						select {
						case <-nodeCtx.Done():
							n.log("Node(%s/%s) node-ctx done for Node(%s)", node.key, ingressLink.y, ingressLink.x)
							return nil
						case inData := <-ingressLink.ch:
							ingressLink.tally++
							n.log("Node(%s/%s) Received Data(%v) From(%s)", node.key, ingressLink.y, inData, ingressLink.x)

							nodeData, nodeErr := node.f(nodeCtx, inData)
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
								case <-nodeCtx.Done():
									n.log("Node(%s/%s) node-ctx done while distributing Data(%v) Of(%s) To Nodes(%s)", node.key, egressLink.x, nodeData, ingressLink.x, egressYs)
									return nil
								case egressLink.ch <- nodeData:
									n.log("Node(%s/%s) Distributed Data(%v) Of(%s) To Nodes(%s)", node.key, egressLink.x, nodeData, ingressLink.y, egressYs)
								}
							} else {
								for _, egressLink := range egress {
									n.log("Node(%s/%s) Sending Data(%v) Of(%s) To Node(%s)", node.key, egressLink.x, nodeData, ingressLink.x, egressLink.y)
									select {
									case <-nodeCtx.Done():
										n.log("Node(%s/%s) node-ctx done while sending Data(%v) Of(%s) To Node(%s)", node.key, egressLink.x, nodeData, ingressLink.x, egressLink.y)
										return nil
									case egressLink.ch <- nodeData:
										n.log("Node(%s/%s) Sent Data(%v) Of(%s) To Node(%s)", node.key, egressLink.x, nodeData, ingressLink.y, egressLink.y)
									}
								}
							}
						}
					}
				}
				return nil
			})
		}

		if err := nodeWg.Wait(); err != nil {
			return err
		}
	case len(ingress) > 0 && len(egress) == 0:
		n.log("Terminus Node(%s) is running", node.key)

		nodeWg, nodeCtx := errgroup.WithContext(ctx)

		for _, ingressLink := range ingress {
			nodeWg.Go(func() error {
				for {
					select {
					case <-nodeCtx.Done():
						n.log("Terminus(%s/%s) node-ctx done for Node(%s)", node.key, ingressLink.y, ingressLink.x)
						return nil
					case inData := <-ingressLink.ch:
						ingressLink.tally++
						n.log("Terminus(%s/%s) Received Data(%v) From(%s)", node.key, ingressLink.y, inData, ingressLink.x)

						nodeData, nodeErr := node.f(nodeCtx, inData)
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

		if err := nodeWg.Wait(); err != nil {
			return err
		}
	}

	return nil
}
