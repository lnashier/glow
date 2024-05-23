package glow

import (
	"sync"
	"time"
)

// Link captures the connection between two nodes.
// Data flows from x to y over the Link.
// A Link can have 3 states:
//   - Closed - The Link enters the closed state when the X Node goes away.
//     The Link remains in this state until a new session (Network.Start) or system restart.
//   - Paused - The Link enters the paused state when Network.PauseLink is called.
//     The Link remains in this state until resumed (Network.ResumeLink) or system restart.
//   - Removed - The Link enters the removed state when Network.RemoveLink is called.
//     The Link remains in this state until added back (Network.AddLink) or system restart.
type Link struct {
	x       *Node
	y       *Node
	ch      *channel
	tally   int
	once    sync.Once
	closed  bool
	paused  bool
	removed bool
}

type channel struct {
	ch   chan any
	size int
}

type LinkOpt func(*Link)

func (l *Link) apply(opt ...LinkOpt) {
	for _, o := range opt {
		o(l)
	}
}

// Size sets bandwidth for the Link.
func Size(k int) LinkOpt {
	return func(l *Link) {
		l.ch.size = k
	}
}

// From returns the key of the "from" Node connected by this Link.
func (l *Link) From() *Node {
	return l.x
}

// To returns the key of the "to" Node connected by this Link.
func (l *Link) To() *Node {
	return l.y
}

// Tally returns the total count of data transmitted over the link thus far.
func (l *Link) Tally() int {
	return l.tally
}

func (l *Link) Uptime() time.Duration {
	if l.removed || l.paused {
		return 0
	}
	if !l.x.session.start.IsZero() && !l.y.session.start.IsZero() {
		stop := l.x.session.stop
		if stop.IsZero() {
			stop = l.y.session.stop
		}
		if stop.IsZero() {
			stop = time.Now()
		}
		return stop.Sub(l.y.session.start)
	}

	return 0
}

// AddLink connects from-node to to-node.
// Once Link is made, nodes are said to be communicating over the Link in the direction from -> to.
// See:
//   - RemoveLink
//   - PauseLink
//   - ResumeLink
func (n *Network) AddLink(from, to string, opt ...LinkOpt) error {
	if link, _ := n.link(from, to); link != nil {
		if link.removed {
			return ErrNetworkNeedPurging
		}
		return ErrLinkAlreadyExists
	}

	xNode, err := n.Node(from)
	if err != nil {
		return err
	}
	yNode, err := n.Node(to)
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

	link := &Link{
		x:  xNode,
		y:  yNode,
		ch: &channel{},
	}
	link.apply(opt...)

	if xNode.distributor {
		// if there exists another egress for node-x then get existing channel
		if xEgress := n.egress[xNode.Key()]; len(xEgress) > 0 {
			for _, xLink := range xEgress {
				link.ch = xLink.ch
				break
			}
		}
	}

	if link.ch.ch == nil {
		link.ch.ch = make(chan any, link.ch.size)
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

// RemoveLink disconnects "from" Node and "to" Node.
// See:
//   - AddLink
//   - PauseLink
//   - ResumeLink
func (n *Network) RemoveLink(from, to string) error {
	link, err := n.Link(from, to)
	if err != nil {
		return err
	}

	// check if session is in progress
	n.session.mu.RLock()
	defer n.session.mu.RUnlock()

	n.mu.Lock()
	defer n.mu.Unlock()

	link.removed = true

	return nil
}

func (n *Network) removeLink(l *Link) error {
	delete(n.ingress[l.y.Key()], l.x.Key())
	delete(n.egress[l.x.Key()], l.y.Key())
	return nil
}

// PauseLink pauses communication from Node and to Node.
// See:
//   - AddLink
//   - ResumeLink
//   - RemoveLink
func (n *Network) PauseLink(from, to string) error {
	link, err := n.Link(from, to)
	if err != nil {
		return err
	}

	if link.paused {
		// parity with AddLink
		return ErrLinkAlreadyPaused
	}

	// check if session is in progress
	n.session.mu.RLock()
	defer n.session.mu.RUnlock()

	n.mu.Lock()
	defer n.mu.Unlock()

	link.paused = true

	return nil
}

// ResumeLink resumes communication from node and to node.
// See:
//   - PauseLink
func (n *Network) ResumeLink(from, to string) error {
	link, err := n.Link(from, to)
	if err != nil {
		return err
	}

	// check if session is in progress
	n.session.mu.RLock()
	defer n.session.mu.RUnlock()

	n.mu.Lock()
	defer n.mu.Unlock()

	link.paused = false

	return nil
}

// Link returns connection between from and to nodes if any.
func (n *Network) Link(from, to string) (*Link, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	link, err := n.link(from, to)
	if err != nil {
		return nil, err
	}
	if link.removed {
		return nil, ErrLinkNotFound
	}

	return link, nil
}

func (n *Network) link(from, to string) (*Link, error) {
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
func (n *Network) Ingress(key string) []*Link {
	n.mu.RLock()
	defer n.mu.RUnlock()

	ingress := n.ingress[key]
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
func (n *Network) Egress(key string) []*Link {
	n.mu.RLock()
	defer n.mu.RUnlock()

	egress := n.egress[key]
	if len(egress) < 1 {
		return nil
	}

	var links []*Link
	for _, link := range egress {
		links = append(links, link)
	}

	return links
}

// closeEgress closes all outgoing links for the Node.
func (n *Network) closeEgress(node *Node) {
	for _, link := range n.Egress(node.Key()) {
		link.once.Do(func() {
			close(link.ch.ch)
			link.closed = true
		})
	}
}

// refreshEgress opens all outgoing links for the Node.
func (n *Network) refreshEgress(node *Node) {
	for _, link := range n.Egress(node.Key()) {
		if link.closed {
			link.closed = false
			link.once = sync.Once{}
			link.ch.ch = make(chan any, link.ch.size)
		}
	}
}
