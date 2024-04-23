package glow

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

var (
	ErrNodeNotFound      = errors.New("node not found")
	ErrBadNodeKey        = errors.New("bad node key")
	ErrNodeAlreadyExists = errors.New("node already exists")
	ErrLinkNotFound      = errors.New("link not found")
	ErrLinkAlreadyExists = errors.New("link already exists")
	ErrNodeIsConnected   = errors.New("node is connected")
	ErrEmptyNetwork      = errors.New("network is empty")
	ErrSeedingDone       = errors.New("seeding is done")
	ErrUnlinkedNodeFound = errors.New("unlinked node found")
)

// Node defines a node in the [Network].
// Node is said to be Seed node if it has only egress Links.
// Node is said to be Terminus node if it has only ingress Links.
type Node func(context.Context, []byte) ([]byte, error)

// Link captures connection between two nodes.
// Data flows from x to y over the [Link].
type Link struct {
	X  string
	Y  string
	ch chan []byte
}

// Network represents nodes and their links.
type Network struct {
	ctx     context.Context
	cancel  func()
	log     func(format string, a ...any)
	lock    *sync.RWMutex
	nodes   map[string]Node             // stores all nodes
	ingress map[string]map[string]*Link // stores all ingress links for all nodes.
	egress  map[string]map[string]*Link // stores all egress links for all nodes.
}

// New creates a new [Network].
// Provided [Key] function is used to get unique keys for the nodes.
func New(opt ...NetworkOpt) *Network {
	opts := defaultNetworkOpts
	opts.apply(opt)

	ctx, cancel := context.WithCancel(context.Background())
	if opts.stopGracetime > 0 {
		cancel1 := cancel
		cancel = func() {
			time.Sleep(opts.stopGracetime)
			cancel1()
		}
	}

	return &Network{
		ctx:    ctx,
		cancel: cancel,
		lock:   &sync.RWMutex{},
		log: func(format string, a ...any) {
			if opts.verbose {
				fmt.Println(fmt.Sprintf("[%s] %s", time.Now().Format("2006-01-02 15:04:05.000"), fmt.Sprintf(format, a...)))
			}
		},
		nodes:   make(map[string]Node),
		ingress: make(map[string]map[string]*Link),
		egress:  make(map[string]map[string]*Link),
	}
}

// AddNode adds a new Node in the network.
// Node key is retrieved from the provided [Key] function.
func (n *Network) AddNode(node Node, opt ...NodeOpt) (string, error) {
	n.lock.Lock()
	defer n.lock.Unlock()

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

	n.nodes[k] = node

	return k, nil
}

// Node returns the node identified by the provided key.
func (n *Network) Node(k string) (Node, error) {
	n.lock.RLock()
	defer n.lock.RUnlock()

	node, ok := n.nodes[k]
	if !ok {
		return node, ErrNodeNotFound
	}

	return node, nil
}

// RemoveNode removes a node with provided key.
// A node can't be removed if it is connected/linked to any other node in the network.
func (n *Network) RemoveNode(k string) error {
	n.lock.Lock()
	defer n.lock.Unlock()

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

// Nodes returns all the nodes as their unique keys in the network.
// Node should be called to get actual node.
func (n *Network) Nodes() []string {
	n.lock.RLock()
	defer n.lock.RUnlock()

	keys := make([]string, 0, len(n.nodes))
	for k := range n.nodes {
		keys = append(keys, k)
	}

	return keys
}

// Seeds returns all the nodes that have only egress links.
// Node should be called to get actual node.
func (n *Network) Seeds() []string {
	n.lock.RLock()
	defer n.lock.RUnlock()

	var keys []string

	for k := range n.nodes {
		ingress := n.Ingress(k)
		egress := n.Egress(k)
		if len(ingress) == 0 && len(egress) > 0 {
			keys = append(keys, k)
		}
	}

	return keys
}

// Termini returns all the nodes that have only ingress links.
// Node should be called to get actual node.
func (n *Network) Termini() []string {
	n.lock.RLock()
	defer n.lock.RUnlock()

	var keys []string

	for k := range n.nodes {
		ingress := n.Ingress(k)
		egress := n.Egress(k)
		if len(ingress) > 0 && len(egress) == 0 {
			keys = append(keys, k)
		}
	}

	return keys
}

// AddLink connects from node and to nodes.
// Once Link is made, nodes are said to be communicated over Link channel.
func (n *Network) AddLink(from, to string, size int) error {
	if _, err := n.Link(from, to); !errors.Is(err, ErrLinkNotFound) {
		return ErrLinkAlreadyExists
	}

	_, err := n.Node(from)
	if err != nil {
		return err
	}
	_, err = n.Node(to)
	if err != nil {
		return err
	}

	n.lock.Lock()
	defer n.lock.Unlock()

	link := &Link{
		X:  from,
		Y:  to,
		ch: make(chan []byte, size),
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

// Link returns connection between from and to nodes if any.
func (n *Network) Link(from, to string) (*Link, error) {
	n.lock.RLock()
	defer n.lock.RUnlock()

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

// RemoveLink disconnects from node and to node.
// Underlying communication channel is closed.
func (n *Network) RemoveLink(from, to string) error {
	_, err := n.Link(from, to)
	if err != nil {
		return err
	}

	n.lock.Lock()
	defer n.lock.Unlock()

	delete(n.ingress[to], from)
	delete(n.egress[from], to)

	return nil
}

// Links returns all the links in the Network.
func (n *Network) Links() []*Link {
	n.lock.RLock()
	defer n.lock.RUnlock()

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
	n.lock.RLock()
	defer n.lock.RUnlock()

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
	n.lock.RLock()
	defer n.lock.RUnlock()

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
	n.log("Start enter")
	defer n.log("Start exit")

	keys := n.Nodes()
	if len(keys) == 0 {
		return ErrEmptyNetwork
	}

	n.log("Nodes: %v", keys)

	wg, netctx := errgroup.WithContext(n.ctx)

	for _, key := range keys {
		wg.Go(func() error {
			node, err := n.Node(key)
			if err != nil {
				return err
			}

			ingress := n.Ingress(key)
			egress := n.Egress(key)

			if len(ingress) == 0 && len(egress) == 0 {
				return ErrUnlinkedNodeFound
			}

			n.log("Node(%s) ingress(%v) egress(%v)", key, ingress, egress)

			switch {
			case len(ingress) == 0 && len(egress) > 0:
				n.log("Seed(%s) running", key)

				for {
					select {
					case <-netctx.Done():
						n.log("Seed(%s) net-ctx done", key)
						return nil
					default:
						nodeData, nodeErr := node(netctx, nil)
						if nodeErr != nil {
							if errors.Is(nodeErr, ErrSeedingDone) {
								n.log("Seed(%s) %v", key, nodeErr)
								return nil
							}
							n.log("Seed(%s) Err: %v", key, nodeErr)
							return nodeErr
						}

						for _, link := range egress {
							n.log("Seed(%s/%s) Sending Data(%s) To Node(%s)", key, link.X, string(nodeData), link.Y)
							select {
							case <-netctx.Done():
								n.log("Seed(%s/%s) net-ctx done while sending Data(%s) To Node(%s)", key, link.X, string(nodeData), link.Y)
								return nil
							case link.ch <- nodeData:
								n.log("Seed(%s/%s) Sent Data(%s) To Node(%s)", key, link.X, string(nodeData), link.Y)
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
								n.log("Node(%s/%s) From(%s) node-ctx done", key, ingressLink.X, ingressLink.Y)
								return nil
							case inData := <-ingressLink.ch:
								n.log("Node(%s/%s) Received Data(%s) From(%s)", key, ingressLink.Y, string(inData), ingressLink.X)

								nodeData, nodeErr := node(nodectx, inData)
								if nodeErr != nil {
									n.log("Node(%s/%s) Err: %v", key, ingressLink.Y, nodeErr)
									return nodeErr
								}

								for _, egressLink := range egress {
									n.log("Node(%s/%s) Sending Data(%s) Of(%s) To Node(%s)", key, egressLink.X, string(nodeData), ingressLink.X, egressLink.Y)
									select {
									case <-nodectx.Done():
										n.log("Node(%s/%s) node-ctx done while sending Data(%s) Of(%s) To Node(%s)", key, egressLink.X, string(nodeData), ingressLink.X, egressLink.Y)
										return nil
									case egressLink.ch <- nodeData:
										n.log("Node(%s/%s) Sent Data(%s) Of(%s) To Node(%s)", key, egressLink.X, string(nodeData), ingressLink.X, egressLink.Y)
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
								n.log("Terminus(%s/%s) From(%s) node-ctx done", key, ingressLink.X, ingressLink.Y)
								return nil
							case inData := <-ingressLink.ch:
								n.log("Terminus(%s/%s) Received Data(%s) From(%s)\n", key, ingressLink.Y, string(inData), ingressLink.X)

								_, nodeErr := node(nodectx, inData)
								if nodeErr != nil {
									n.log("Terminus(%s/%s) Err: %v\n", key, ingressLink.Y, nodeErr)
									return nodeErr
								}
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

// Stop signals the Network to stop all the communications.
func (n *Network) Stop() error {
	n.log("Stop enter")
	defer n.log("Stop exit")
	n.cancel()
	return nil
}
