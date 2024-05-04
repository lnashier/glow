package glow

import (
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
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
	n.log("Network coming up")
	defer n.session.mu.Unlock()
	defer n.log("Network shut down")

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

	wg, ctx := errgroup.WithContext(n.session.ctx)

	for _, key := range keys {
		wg.Go(func() error {
			return n.nodeUp(ctx, key)
		})
	}

	return wg.Wait()
}

// Stop signals the Network to cease all communications.
// If stop grace period is set, communications will terminate
// after that period.
func (n *Network) Stop() error {
	n.log("Stopping network")
	defer n.log("Network signaled to stop")
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
