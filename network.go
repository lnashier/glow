package glow

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
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

// AddLink connects from-node to to-node.
// Once Link is made, nodes are said to be communicating over the Link from -> to.
func (n *Network) AddLink(from, to string, opt ...LinkOpt) error {
	if link, _ := n.Link(from, to); link != nil {
		return ErrLinkAlreadyExists
	}

	xNode, err := n.Node(from)
	if err != nil {
		return err
	}
	_, err = n.Node(to)
	if err != nil {
		return err
	}

	if n.preventCycles && n.checkCycle(from, to) {
		return ErrCyclesNotAllowed
	}

	// check if session is in progress
	n.session.mu.RLock()
	defer n.session.mu.RUnlock()

	n.mu.Lock()
	defer n.mu.Unlock()

	opts := defaultLinkOpts
	opts.apply(opt)

	var ch chan any

	if xNode.distributor {
		// if there exists another egress for node-x then get existing channel
		if xEgress := n.egress[xNode.key]; len(xEgress) > 0 {
			for _, xLink := range xEgress {
				ch = xLink.ch
				break
			}
		}
	}

	if ch == nil {
		ch = make(chan any, opts.size)
	}

	link := &Link{
		x:  from,
		y:  to,
		ch: ch,
	}

	if _, ok := n.egress[from]; !ok {
		n.egress[from] = make(map[string]*Link)
	}

	n.egress[from][to] = link

	if _, ok := n.ingress[to]; !ok {
		n.ingress[to] = make(map[string]*Link)
	}

	n.ingress[to][from] = link

	return nil
}

// RemoveLink disconnects "from" node and "to" node.
func (n *Network) RemoveLink(from, to string) error {
	_, err := n.Link(from, to)
	if err != nil {
		return err
	}

	// check if session is in progress
	n.session.mu.RLock()
	defer n.session.mu.RUnlock()

	n.mu.Lock()
	defer n.mu.Unlock()

	delete(n.ingress[to], from)
	delete(n.egress[from], to)

	return nil
}

// Link returns connection between from and to nodes if any.
func (n *Network) Link(from, to string) (*Link, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	outLinks, ok := n.egress[from]
	if !ok {
		return nil, ErrLinkNotFound
	}

	link, ok := outLinks[to]
	if !ok {
		return nil, ErrLinkNotFound
	}

	return link, nil
}

// Links returns all the links in the Network.
func (n *Network) Links() []*Link {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var links []*Link
	for _, outLinks := range n.egress {
		for _, link := range outLinks {
			links = append(links, link)
		}
	}

	return links
}

// Ingress returns all the ingress links for the Node.
func (n *Network) Ingress(k string) []*Link {
	n.mu.RLock()
	defer n.mu.RUnlock()

	ingress := n.ingress[k]
	if len(ingress) < 1 {
		return nil
	}

	var links []*Link
	for _, link := range ingress {
		links = append(links, link)
	}

	return links
}

// Egress returns all the egress links for the Node.
func (n *Network) Egress(k string) []*Link {
	n.mu.RLock()
	defer n.mu.RUnlock()

	egress := n.egress[k]
	if len(egress) < 1 {
		return nil
	}

	var links []*Link
	for _, link := range egress {
		links = append(links, link)
	}

	return links
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
			node, err := n.Node(key)
			if err != nil {
				return err
			}

			ingress := n.Ingress(key)
			egress := n.Egress(key)

			if len(ingress) == 0 && len(egress) == 0 {
				if n.ignoreIsolatedNodes {
					return nil
				}
				return ErrIsolatedNodeFound
			}

			n.log("Node(%s) ingress(%v) egress(%v)", key, ingress, egress)

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

				err = nodewg.Wait()
				if err != nil {
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

				err = nodewg.Wait()
				if err != nil {
					return err
				}
			}

			return nil
		})
	}

	return wg.Wait()
}

// Stop signals the Network to cease all communications.
// If a stop grace period is set, communications will terminate
// gracefully after that period.
func (n *Network) Stop() error {
	n.log("Stop enter")
	defer n.log("Stop exit")
	if n.session.cancel != nil {
		n.session.cancel()
	}
	return nil
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
