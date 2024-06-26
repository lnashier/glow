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

type NetworkOpt func(*Network)

// Verbose enables Network to send logs to stdout.
func Verbose() NetworkOpt {
	return func(n *Network) {
		n.log = func(format string, a ...any) {
			fmt.Println(fmt.Sprintf("[%s] %s", time.Now().Format("2006-01-02 15:04:05.000"), fmt.Sprintf(format, a...)))
		}
	}
}

// IgnoreIsolatedNodes allows the Network to run even when there are isolated nodes.
func IgnoreIsolatedNodes() NetworkOpt {
	return func(n *Network) {
		n.ignoreIsolatedNodes = true
	}
}

// StopGracetime sets the grace period for which the Network waits for processes to finish before stopping.
func StopGracetime(t time.Duration) NetworkOpt {
	return func(n *Network) {
		n.stopGracetime = t
	}
}

// PreventCycles ensures the Network remains a Directed Acyclic Graph (DAG).
func PreventCycles() NetworkOpt {
	return func(n *Network) {
		n.preventCycles = true
	}
}

// New creates a new [Network].
func New(opt ...NetworkOpt) *Network {
	net := &Network{
		mu: &sync.RWMutex{},
		session: &session{
			mu: &sync.RWMutex{},
		},
		log:     func(format string, a ...any) {},
		nodes:   make(map[string]*Node),
		ingress: make(map[string]map[string]*Link),
		egress:  make(map[string]map[string]*Link),
	}
	net.apply(opt...)
	return net
}

// Start runs the Network.
func (n *Network) Start(ctx context.Context) error {
	n.session.mu.Lock()
	n.log("Network coming up")
	defer n.session.mu.Unlock()
	defer n.log("Network shut down")

	n.session.start = time.Now()
	n.session.stop = time.Time{} //unset
	defer func() {
		n.session.stop = time.Now()
	}()
	n.session.ctx, n.session.cancel = context.WithCancel(ctx)
	if n.stopGracetime > 0 {
		cancel1 := n.session.cancel
		n.session.cancel = func() {
			n.log("Network going down in %s", n.stopGracetime)
			time.Sleep(n.stopGracetime)
			cancel1()
		}
	}

	n.refreshNodes()

	nodes := n.Nodes()
	if len(nodes) == 0 {
		return ErrEmptyNetwork
	}

	n.log("Nodes: %d", len(nodes))

	wg, netCtx := errgroup.WithContext(n.session.ctx)

	for _, node := range nodes {
		wg.Go(func() error {
			return n.nodeUp(netCtx, node)
		})
	}

	return wg.Wait()
}

// Stop signals the Network to cease all communications.
// If stop grace period is set, communications will terminate after that period.
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

	keys := n.Keys()
	links := n.Links()

	n.mu.Lock()
	defer n.mu.Unlock()

	// clean up removed links
	for _, link := range links {
		if link.removed {
			err := n.removeLink(link)
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

func (n *Network) Uptime() time.Duration {
	if n.session.start.IsZero() {
		return 0
	}
	if n.session.stop.IsZero() {
		return time.Since(n.session.start)
	}
	return n.session.stop.Sub(n.session.start)
}

func (n *Network) apply(opt ...NetworkOpt) {
	for _, o := range opt {
		o(n)
	}
}

type session struct {
	mu     *sync.RWMutex
	ctx    context.Context
	cancel func()
	start  time.Time
	stop   time.Time
}
