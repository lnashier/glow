package fan1seednet

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	xtime "github.com/lnashier/goarc/x/time"
	"strconv"
	"sync"
	"time"
)

var seedCounts sync.Map
var nodeInCounts sync.Map
var nodeOutCounts sync.Map

func Network() *glow.Network {
	nodeCount := 0
	keygen := func() string {
		nodeCount++
		return fmt.Sprintf("node-%d", nodeCount)
	}

	n := glow.New(glow.Verbose())

	for i := range 1 {
		seedCounts.Store(i, i*100)
	}

	for i := range 7 {
		nodeInCounts.Store(i, []string{})
		nodeOutCounts.Store(i, []string{})
	}

	node0, err := n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		xtime.SleepWithContext(ctx, time.Second*5)

		num, _ := seedCounts.Load(0)
		seedCounts.Store(0, num.(int)+1)

		inCounts, _ := nodeInCounts.Load(0)
		nodeInCounts.Store(0, append(inCounts.([]string), string(in)))

		defer func() {
			outCounts, _ := nodeOutCounts.Load(0)
			nodeOutCounts.Store(0, append(outCounts.([]string), strconv.Itoa(num.(int)+1)))
		}()

		return []byte(fmt.Sprintf("%d", num.(int)+1)), nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node1, err := n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		inCounts, _ := nodeInCounts.Load(1)
		nodeInCounts.Store(1, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(1)
			nodeOutCounts.Store(1, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node2, err := n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		inCounts, _ := nodeInCounts.Load(2)
		nodeInCounts.Store(2, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(2)
			nodeOutCounts.Store(2, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node3, err := n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		inCounts, _ := nodeInCounts.Load(3)
		nodeInCounts.Store(3, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(3)
			nodeOutCounts.Store(3, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node4, err := n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		inCounts, _ := nodeInCounts.Load(4)
		nodeInCounts.Store(4, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(4)
			nodeOutCounts.Store(4, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node5, err := n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		inCounts, _ := nodeInCounts.Load(5)
		nodeInCounts.Store(5, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(5)
			nodeOutCounts.Store(5, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node6, err := n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		inCounts, _ := nodeInCounts.Load(6)
		nodeInCounts.Store(6, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(6)
			nodeOutCounts.Store(6, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	size := 0

	err = n.AddLink(node0, node1, glow.Size(size))
	if err != nil {
		panic(err)
	}

	err = n.AddLink(node0, node2, glow.Size(size))
	if err != nil {
		panic(err)
	}

	err = n.AddLink(node1, node3, glow.Size(size))
	if err != nil {
		panic(err)
	}

	err = n.AddLink(node1, node4, glow.Size(size))
	if err != nil {
		panic(err)
	}

	err = n.AddLink(node2, node5, glow.Size(size))
	if err != nil {
		panic(err)
	}

	err = n.AddLink(node2, node6, glow.Size(size))
	if err != nil {
		panic(err)
	}

	return n
}

func PrintResults() {
	seedCounts.Range(func(k, v any) bool {
		fmt.Printf("seedCounts[seed-%d] = %d\n", k.(int)+1, v)
		return true
	})

	nodeInCounts.Range(func(k, v any) bool {
		fmt.Printf("nodeInCounts [node-%d] = %v\n", k.(int)+1, v)
		nc, _ := nodeOutCounts.Load(k)
		fmt.Printf("nodeOutCounts[node-%d] = %v\n", k.(int)+1, nc)
		return true
	})
}
