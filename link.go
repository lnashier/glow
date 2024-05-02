package glow

// Link captures connection between two nodes.
// Data flows from x to y over the [Link].
type Link struct {
	x       string
	y       string
	ch      chan any
	tally   int
	paused  bool
	deleted bool
}

// From returns the key of the "from" Node connected by this link.
func (l *Link) From() string {
	return l.x
}

// To returns the key of the "to" Node connected by this link.
func (l *Link) To() string {
	return l.y
}

// Tally returns the total count of data transmitted over the link thus far.
func (l *Link) Tally() int {
	return l.tally
}

type LinkOpt func(*linkOpts)

type linkOpts struct {
	size int
}

var defaultLinkOpts = linkOpts{}

func (s *linkOpts) apply(opts []LinkOpt) {
	for _, o := range opts {
		o(s)
	}
}

// Size sets bandwidth for the Link.
func Size(k int) LinkOpt {
	return func(s *linkOpts) {
		s.size = k
	}
}

// AddLink connects from-node to to-node.
// Once Link is made, nodes are said to be communicating over the Link from -> to.
// See:
//   - RemoveLink
//   - PauseLink
//   - ResumeLink
func (n *Network) AddLink(from, to string, opt ...LinkOpt) error {
	if link, _ := n.link(from, to); link != nil {
		if link.deleted {
			return ErrNetworkNeedPurging
		}
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

	link.deleted = true

	return nil
}

func (n *Network) removeLink(from, to string) error {
	_, err := n.link(from, to)
	if err != nil {
		return err
	}
	delete(n.ingress[to], from)
	delete(n.egress[from], to)
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
	if link.deleted {
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