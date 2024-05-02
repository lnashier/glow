package glow

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
	"slices"
	"strings"
	"sync"
	"time"
)

// Network represents nodes and their links.
type Network struct {
	mu                  *sync.RWMutex
	session             *session
	log                 func(format string, a ...any)
	nodes               map[string]*Node            // stores all nodes
	ingress             map[string]map[string]*Link // stores all ingress links for all nodes.
	egress              map[string]map[string]*Link // stores all egress links for all nodes.
	stopGracetime       time.Duration
	ignoreIsolatedNodes bool
	preventCycles       bool
}

type session struct {
	mu     *sync.RWMutex
	ctx    context.Context
	cancel func()
}

type NetworkOpt func(*networkOpts)

type networkOpts struct {
	verbose             bool
	stopGracetime       time.Duration
	ignoreIsolatedNodes bool
	preventCycles       bool
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

func PreventCycles() NetworkOpt {
	return func(s *networkOpts) {
		s.preventCycles = true
	}
}

// New creates a new [Network].
func New(opt ...NetworkOpt) *Network {
	opts := defaultNetworkOpts
	opts.apply(opt)

	return &Network{
		mu: &sync.RWMutex{},
		session: &session{
			mu: &sync.RWMutex{},
		},
		log: func(format string, a ...any) {
			if opts.verbose {
				fmt.Println(fmt.Sprintf("[%s] %s", time.Now().Format("2006-01-02 15:04:05.000"), fmt.Sprintf(format, a...)))
			}
		},
		nodes:               make(map[string]*Node),
		ingress:             make(map[string]map[string]*Link),
		egress:              make(map[string]map[string]*Link),
		ignoreIsolatedNodes: opts.ignoreIsolatedNodes,
		stopGracetime:       opts.stopGracetime,
		preventCycles:       opts.preventCycles,
	}
}

// Start runs the Network.
func (n *Network) Start() error {
	n.session.mu.Lock()
	n.log("Start enter")
	defer n.session.mu.Unlock()
	defer n.log("Start exit")

	n.session.ctx, n.session.cancel = context.WithCancel(context.Background())
	if n.stopGracetime > 0 {
		cancel1 := n.session.cancel
		n.session.cancel = func() {
			n.log("Network going down in %s", n.stopGracetime)
			time.Sleep(n.stopGracetime)
			cancel1()
		}
	}

	keys := n.Nodes()
	if len(keys) == 0 {
		return ErrEmptyNetwork
	}

	n.log("Nodes: %v", keys)

	wg, netctx := errgroup.WithContext(n.session.ctx)

	for _, key := range keys {
		wg.Go(func() error {
			n.log("Node(%s) coming up", key)
			defer n.log("Node(%s) going down", key)

			node, err := n.Node(key)
			if err != nil {
				return err
			}

			ingress := slices.DeleteFunc(n.Ingress(key), func(l *Link) bool {
				return l.paused || l.deleted
			})
			egress := slices.DeleteFunc(n.Egress(key), func(l *Link) bool {
				return l.paused || l.deleted
			})

			n.log("Node(%s) ingress(%v) egress(%v)", key, ingress, egress)

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
				n.log("Seed(%s) running", key)

				for {
					select {
					case <-netctx.Done():
						n.log("Seed(%s) net-ctx done", key)
						return nil
					default:
						nodeData, nodeErr := node.f(netctx, nil)
						if nodeErr != nil {
							if errors.Is(nodeErr, ErrSeedingDone) || errors.Is(nodeErr, ErrNodeGoingAway) {
								n.log("Seed(%s) %v", key, nodeErr)
								return nil
							}
							n.log("Seed(%s) Err: %v", key, nodeErr)
							return nodeErr
						}

						if node.distributor {
							// Get any egress link, they all share same channel
							egressLink := egress[0]
							n.log("Seed(%s/%s) Distributing Data(%v) To Nodes(%s)", key, egressLink.x, nodeData, egressYs)
							select {
							case <-netctx.Done():
								n.log("Seed(%s/%s) net-ctx done while distributing Data(%v) To Nodes(%s)", key, egressLink.x, nodeData, egressYs)
								return nil
							case egressLink.ch <- nodeData:
								n.log("Seed(%s/%s) Distributed Data(%v) To Nodes(%s)", key, egressLink.x, nodeData, egressYs)
							}
						} else {
							for _, egressLink := range egress {
								n.log("Seed(%s/%s) Sending Data(%v) To Node(%s)", key, egressLink.x, nodeData, egressLink.y)
								select {
								case <-netctx.Done():
									n.log("Seed(%s/%s) net-ctx done while sending Data(%v) To Node(%s)", key, egressLink.x, nodeData, egressLink.y)
									return nil
								case egressLink.ch <- nodeData:
									n.log("Seed(%s/%s) Sent Data(%v) To Node(%s)", key, egressLink.x, nodeData, egressLink.y)
								}
							}
						}
					}
				}
			case len(ingress) > 0 && len(egress) > 0:
				n.log("Node(%s) running", key)

				nodewg, nodectx := errgroup.WithContext(netctx)

				for _, ingressLink := range ingress {
					nodewg.Go(func() error {
						for {
							select {
							case <-nodectx.Done():
								n.log("Node(%s/%s) node-ctx done for Node(%s)", key, ingressLink.y, ingressLink.x)
								return nil
							case inData := <-ingressLink.ch:
								ingressLink.tally++
								n.log("Node(%s/%s) Received Data(%v) From(%s)", key, ingressLink.y, inData, ingressLink.x)

								nodeData, nodeErr := node.f(nodectx, inData)
								if nodeErr != nil {
									if errors.Is(nodeErr, ErrNodeGoingAway) {
										n.log("Node(%s/%s) %v for Node(%s)", key, ingressLink.y, nodeErr, ingressLink.x)
										return nil
									}
									n.log("Node(%s/%s) Err: %v for Node(%s)", key, ingressLink.y, nodeErr, ingressLink.x)
									return nodeErr
								}

								if node.distributor {
									// Get any egress link, they all share same channel
									egressLink := egress[0]
									n.log("Node(%s/%s) Distributing Data(%v) Of(%s) To Nodes(%s)", key, egressLink.x, nodeData, ingressLink.x, egressYs)
									select {
									case <-nodectx.Done():
										n.log("Node(%s/%s) node-ctx done while distributing Data(%v) Of(%s) To Nodes(%s)", key, egressLink.x, nodeData, ingressLink.x, egressYs)
										return nil
									case egressLink.ch <- nodeData:
										n.log("Node(%s/%s) Distributed Data(%v) Of(%s) To Nodes(%s)", key, egressLink.x, nodeData, ingressLink.y, egressYs)
									}
								} else {
									for _, egressLink := range egress {
										n.log("Node(%s/%s) Sending Data(%v) Of(%s) To Node(%s)", key, egressLink.x, nodeData, ingressLink.x, egressLink.y)
										select {
										case <-nodectx.Done():
											n.log("Node(%s/%s) node-ctx done while sending Data(%v) Of(%s) To Node(%s)", key, egressLink.x, nodeData, ingressLink.x, egressLink.y)
											return nil
										case egressLink.ch <- nodeData:
											n.log("Node(%s/%s) Sent Data(%v) Of(%s) To Node(%s)", key, egressLink.x, nodeData, ingressLink.y, egressLink.y)
										}
									}
								}
							}
						}
					})
				}

				if err = nodewg.Wait(); err != nil {
					return err
				}
			case len(ingress) > 0 && len(egress) == 0:
				n.log("Terminus(%s) running", key)

				nodewg, nodectx := errgroup.WithContext(netctx)

				for _, ingressLink := range ingress {
					nodewg.Go(func() error {
						for {
							select {
							case <-nodectx.Done():
								n.log("Terminus(%s/%s) node-ctx done for Node(%s)", key, ingressLink.y, ingressLink.x)
								return nil
							case inData := <-ingressLink.ch:
								ingressLink.tally++
								n.log("Terminus(%s/%s) Received Data(%v) From(%s)", key, ingressLink.y, inData, ingressLink.x)

								nodeData, nodeErr := node.f(nodectx, inData)
								if nodeErr != nil {
									if errors.Is(nodeErr, ErrNodeGoingAway) {
										n.log("Terminus(%s/%s) %v for Node(%s)", key, ingressLink.y, nodeErr, ingressLink.x)
										return nil
									}
									n.log("Terminus(%s/%s) Err: %v for Node(%s)", key, ingressLink.y, nodeErr, ingressLink.x)
									return nodeErr
								}
								n.log("Terminus(%s/%s) Data(%v) for Node(%s)", key, ingressLink.y, nodeData, ingressLink.x)
							}
						}
					})
				}

				if err = nodewg.Wait(); err != nil {
					return err
				}
			}

			return nil
		})
	}

	return wg.Wait()
}

// Stop signals the Network to cease all communications.
// If stop grace period is set, communications will terminate
// after that period.
func (n *Network) Stop() error {
	n.log("Stop enter")
	defer n.log("Stop exit")
	if n.session.cancel != nil {
		n.session.cancel()
	}
	return nil
}

// Purge cleans up the Network by removing isolated Node(s) and removed Link(s).
func (n *Network) Purge() error {
	// check if session is in progress
	n.session.mu.RLock()
	defer n.session.mu.RUnlock()

	keys := n.Nodes()
	links := n.Links()

	n.mu.Lock()
	defer n.mu.Unlock()

	// clean up removed links
	for _, link := range links {
		if link.deleted {
			err := n.removeLink(link.x, link.y)
			if err != nil {
				return err
			}
		}
	}

	// now clean up nodes
	for _, key := range keys {
		if len(n.ingress[key]) == 0 && len(n.egress[key]) == 0 {
			err := n.removeNode(key)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
