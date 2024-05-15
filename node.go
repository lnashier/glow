package glow

import (
	"context"
	"errors"
	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"
	"slices"
	"strings"
	"time"
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
//   - By default, a Node operates in "push-pull" mode: the Network pushes data to NodeFunc,
//     and it waits for NodeFunc to return with output data, which is then forwarded to connected Node(s).
//     This behavior can be changed to "push-push" by setting the EmitFunc for the Node.
//     In emitter mode, the Network pushes data to EmitFunc, and the Node emits data back to the Network
//     through the supplied callback emit function.
type Node struct {
	key         string
	f           func(context.Context, any) (any, error)
	ef          func(context.Context, any, func(any)) error
	distributor bool
	session     nodeSession
}

type nodeSession struct {
	start time.Time
	stop  time.Time
}

type NodeOpt func(*Node)

func (n *Node) Key() string {
	return n.key
}

func (n *Node) Uptime() time.Duration {
	if n.session.start.IsZero() {
		return 0
	}
	if n.session.stop.IsZero() {
		return time.Since(n.session.start)
	}
	return n.session.stop.Sub(n.session.start)
}

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

// Distributor enables a Node to distribute incoming data among its outgoing links.
func Distributor() NodeOpt {
	return func(n *Node) {
		n.distributor = true
	}
}

// NodeFunc is responsible for processing incoming data on the Node.
// Output from the Node is forwarded to downstream connected Node(s).
func NodeFunc(f func(ctx context.Context, data any) (any, error)) NodeOpt {
	return func(n *Node) {
		n.f = f
	}
}

// EmitFunc handles processing incoming data on the Node.
// It provides a callback where output data can be optionally emitted.
// Emitted data is forwarded to downstream connected Node(s).
func EmitFunc(f func(ctx context.Context, data any, emit func(any)) error) NodeOpt {
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

	if len(node.Key()) == 0 {
		return node.Key(), ErrBadNodeKey
	}

	if _, ok := n.nodes[node.Key()]; ok {
		return node.Key(), ErrNodeAlreadyExists
	}

	if node.f == nil && node.ef == nil {
		return node.Key(), ErrNodeFunctionMissing
	}
	if node.f != nil && node.ef != nil {
		return node.Key(), ErrTooManyNodeFunction
	}

	n.nodes[node.Key()] = node

	return node.Key(), nil
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

// Nodes returns all the nodes in the Network.
func (n *Network) Nodes() []*Node {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return maps.Values(n.nodes)
}

// Keys returns all the nodes as their unique keys in the Network.
// Network.Node should be called to get actual Node.
func (n *Network) Keys() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	keys := make([]string, 0, len(n.nodes))
	for _, node := range n.Nodes() {
		keys = append(keys, node.Key())
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
	n.log("Node(%s) coming up", node.Key())
	defer n.log("Node(%s) shut down", node.Key())

	ingress := slices.DeleteFunc(n.Ingress(node.Key()), func(l *Link) bool {
		return l.paused || l.deleted
	})
	egress := slices.DeleteFunc(n.Egress(node.Key()), func(l *Link) bool {
		return l.paused || l.deleted
	})

	n.log("Node(%s) ingress(%v) egress(%v)", node.Key(), ingress, egress)

	if len(ingress) == 0 && len(egress) == 0 {
		if n.ignoreIsolatedNodes {
			return nil
		}
		return ErrIsolatedNodeFound
	}

	node.session.start = time.Now()
	node.session.stop = time.Time{}
	defer func() {
		node.session.stop = time.Now()
	}()

	var egressYs string
	for _, egressLink := range egress {
		egressYs += egressLink.y.Key() + ","
	}
	egressYs = strings.TrimSuffix(egressYs, ",")

	switch {
	case len(ingress) == 0 && len(egress) > 0:
		n.log("Seed Node(%s) is running", node.Key())
		defer n.log("Seed Node(%s) going away", node.Key())
		defer n.closeEgress(node.Key())

		// When the seed-node is in emitter mode, the emitter function is called once.
		// The seed-node has the option to emit as many data points as needed during
		// this emission phase. After the Node emitter function returns, the seed-node
		// gracefully shuts down, concluding its emission process. This mode allows
		// the seed-node to continuously emit data points before terminating its execution.
		nf := node.ef
		if nf == nil {
			// When the seed-node is in regular mode, the node function is invoked repeatedly
			// until the seed-node does not return ErrSeedingDone or ErrNodeGoingAway errors.
			// This indicates that the seed-node has completed its seeding process.
			nf = func(ctx context.Context, in any, emit func(any)) error {
				for {
					select {
					case <-ctx.Done():
						n.log("Seed(%s) net-ctx done", node.Key())
						return nil
					default:
						nodeData, nodeErr := node.f(ctx, nil)
						if nodeErr != nil {
							if errors.Is(nodeErr, ErrSeedingDone) || errors.Is(nodeErr, ErrNodeGoingAway) {
								n.log("Seed(%s) %v", node.Key(), nodeErr)
								return nil
							}
							n.log("Seed(%s) Err: %v", node.Key(), nodeErr)
							return nodeErr
						}
						emit(nodeData)
					}
				}
			}
		}

		nodeWg, nodeCtx := errgroup.WithContext(ctx)
		nodeDataCh := make(chan any)

		nodeWg.Go(func() error {
			err := nf(nodeCtx, nil, func(nodeData any) {
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
					n.log("Seed(%s) node-ctx done while reading emitted data", node.Key())
					return nil
				case nodeData, ok := <-nodeDataCh:
					if !ok {
						n.log("Seed(%s) Data Channel Closed", node.Key())
						return nil
					}
					if node.distributor {
						// Get any egress link, they all share same channel
						egressLink := egress[0]
						n.log("Seed(%s/%s) Distributing Data(%v) To Nodes(%s)", node.Key(), egressLink.x.Key(), nodeData, egressYs)
						select {
						case <-ctx.Done():
							n.log("Seed(%s/%s) net-ctx done while distributing Data(%v) To Nodes(%s)", node.Key(), egressLink.x.Key(), nodeData, egressYs)
							return nil
						case egressLink.ch.ch <- nodeData:
							n.log("Seed(%s/%s) Distributed Data(%v) To Nodes(%s)", node.Key(), egressLink.x.Key(), nodeData, egressYs)
						}
					} else {
						n.log("Seed(%s) Broadcasting Data(%v) To Nodes(%s)", node.Key(), nodeData, egressYs)
						for _, egressLink := range egress {
							n.log("Seed(%s/%s) Sending Data(%v) To Node(%s)", node.Key(), egressLink.x.Key(), nodeData, egressLink.y.Key())
							select {
							case <-ctx.Done():
								n.log("Seed(%s/%s) net-ctx done while sending Data(%v) To Node(%s)", node.Key(), egressLink.x.Key(), nodeData, egressLink.y.Key())
								return nil
							case egressLink.ch.ch <- nodeData:
								n.log("Seed(%s/%s) Sent Data(%v) To Node(%s)", node.Key(), egressLink.x.Key(), nodeData, egressLink.y.Key())
							}
						}
					}
				}
			}
		})

		if err := nodeWg.Wait(); err != nil {
			if errors.Is(err, ErrSeedingDone) || errors.Is(err, ErrNodeGoingAway) {
				n.log("Seed(%s) %v", node.Key(), err)
				return nil
			}
			n.log("Seed(%s) Err: %v", node.Key(), err)
			return err
		}
	case len(ingress) > 0 && len(egress) > 0:
		n.log("Transit Node(%s) is running", node.Key())
		defer n.log("Transit Node(%s) going away", node.Key())
		defer n.closeEgress(node.Key())

		nodeWg, nodeCtx := errgroup.WithContext(ctx)

		// When transit-node is in emitter mode, Node emitter function is called for every incoming data point.
		// Transit-node can choose to emit as many data points and return control back to get next incoming data point.
		nf := node.ef
		if nf == nil {
			// Turn regular node function to emitter node function
			nf = func(ctx context.Context, in any, emit func(any)) error {
				out, err := node.f(ctx, in)
				if err != nil {
					return err
				}
				emit(out)
				return nil
			}
		}

		for _, ingressLink := range ingress {
			nodeWg.Go(func() error {
				inDataWg, inDataCtx := errgroup.WithContext(nodeCtx)
				nodeDataCh := make(chan any)

				inDataWg.Go(func() error {
					for {
						select {
						case <-inDataCtx.Done():
							n.log("Node(%s/%s) in-node-ctx done for Node(%s)", node.Key(), ingressLink.y.Key(), ingressLink.x.Key())
							return nil
						case inData, ok := <-ingressLink.ch.ch:
							if !ok {
								n.log("Node(%s/%s) To Node(%s) Link Closed", node.Key(), ingressLink.y.Key(), ingressLink.x.Key())
								close(nodeDataCh)
								return nil
							}
							ingressLink.tally++
							n.log("Node(%s/%s) Received Data(%v) From(%s)", node.Key(), ingressLink.y.Key(), inData, ingressLink.x.Key())

							if err := nf(inDataCtx, inData, func(nodeData any) {
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
							n.log("Node(%s/%s) node-ctx done for Node(%s) while reading emitted data", node.Key(), ingressLink.y.Key(), ingressLink.x.Key())
							return nil
						case nodeData, ok := <-nodeDataCh:
							if !ok {
								n.log("Node(%s/%s) Data Channel Closed for Node(%s)", node.Key(), ingressLink.y.Key(), ingressLink.x.Key())
								return nil
							}

							if node.distributor {
								// Get any egress link, they all share same channel
								egressLink := egress[0]
								n.log("Node(%s/%s) Distributing Data(%v) Of(%s) To Nodes(%s)", node.Key(), egressLink.x.Key(), nodeData, ingressLink.x.Key(), egressYs)
								select {
								case <-nodeCtx.Done():
									n.log("Node(%s/%s) node-ctx done while distributing Data(%v) Of(%s) To Nodes(%s)", node.Key(), egressLink.x.Key(), nodeData, ingressLink.x.Key(), egressYs)
									return nil
								case egressLink.ch.ch <- nodeData:
									n.log("Node(%s/%s) Distributed Data(%v) Of(%s) To Nodes(%s)", node.Key(), egressLink.x.Key(), nodeData, ingressLink.y.Key(), egressYs)
								}
							} else {
								for _, egressLink := range egress {
									n.log("Node(%s/%s) Sending Data(%v) Of(%s) To Node(%s)", node.Key(), egressLink.x.Key(), nodeData, ingressLink.x.Key(), egressLink.y.Key())
									select {
									case <-nodeCtx.Done():
										n.log("Node(%s/%s) node-ctx done while sending Data(%v) Of(%s) To Node(%s)", node.Key(), egressLink.x.Key(), nodeData, ingressLink.x.Key(), egressLink.y.Key())
										return nil
									case egressLink.ch.ch <- nodeData:
										n.log("Node(%s/%s) Sent Data(%v) Of(%s) To Node(%s)", node.Key(), egressLink.x.Key(), nodeData, ingressLink.y.Key(), egressLink.y.Key())
									}
								}
							}
						}
					}
				})

				if err := inDataWg.Wait(); err != nil {
					if errors.Is(err, ErrNodeGoingAway) {
						n.log("Node(%s/%s) %v for Node(%s)", node.Key(), ingressLink.y.Key(), err, ingressLink.x.Key())
						return nil
					}
					n.log("Node(%s/%s) Err: %v for Node(%s)", node.Key(), ingressLink.y.Key(), err, ingressLink.x.Key())
					return err
				}

				return nil
			})
		}

		if err := nodeWg.Wait(); err != nil {
			return err
		}
	case len(ingress) > 0 && len(egress) == 0:
		n.log("Terminus Node(%s) is running", node.Key())
		defer n.log("Terminus Node(%s) going away", node.Key())

		nodeWg, nodeCtx := errgroup.WithContext(ctx)

		nf := node.ef
		if nf == nil {
			nf = func(ctx context.Context, in any, emit func(any)) error {
				// There is nowhere to send output of node function.
				// Therefore, output of terminal node is ignored.
				_, err := node.f(ctx, in)
				return err
			}
		}

		for _, ingressLink := range ingress {
			nodeWg.Go(func() error {
				for {
					select {
					case <-nodeCtx.Done():
						n.log("Terminus(%s/%s) node-ctx done for Node(%s)", node.Key(), ingressLink.y.Key(), ingressLink.x.Key())
						return nil
					case inData, ok := <-ingressLink.ch.ch:
						if !ok {
							n.log("Terminus(%s/%s) To Node(%s) Link Closed", node.Key(), ingressLink.y.Key(), ingressLink.x.Key())
							return nil
						}
						ingressLink.tally++
						n.log("Terminus(%s/%s) Received Data(%v) From(%s)", node.Key(), ingressLink.y.Key(), inData, ingressLink.x.Key())

						nodeErr := nf(nodeCtx, inData, func(any) {})
						if nodeErr != nil {
							if errors.Is(nodeErr, ErrNodeGoingAway) {
								n.log("Terminus(%s/%s) %v for Node(%s)", node.Key(), ingressLink.y.Key(), nodeErr, ingressLink.x.Key())
								return nil
							}
							n.log("Terminus(%s/%s) Err: %v for Node(%s)", node.Key(), ingressLink.y.Key(), nodeErr, ingressLink.x.Key())
							return nodeErr
						}
						n.log("Terminus(%s/%s) Consumed Data(%v) for Node(%s)", node.Key(), ingressLink.y.Key(), inData, ingressLink.x.Key())
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

func (n *Network) refreshNodes() {
	for _, node := range n.Nodes() {
		node.session = nodeSession{}
		n.refreshLinks(node)
	}
}
