package subwaynet

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	"github.com/lnashier/goarc"
	xtime "github.com/lnashier/goarc/x/time"
	"strconv"
	"sync"
	"time"
)

var seedCounts sync.Map
var nodeInCounts sync.Map
var nodeOutCounts sync.Map

func Run() {
	goarc.Up(Network())
	PrintResults()
}

func Network() *glow.Network {
	n := glow.New(glow.Verbose())

	Sub1Network(n)
	Sub2Network(n)

	// Uncomment to connect two networks
	/*
		err := n.AddLink("node-102", "node-201")
		if err != nil {
			panic(err)
		}
	*/

	return n
}

func Sub1Network(n *glow.Network) {
	nodeCount := 99
	keygen := func() string {
		nodeCount++
		return fmt.Sprintf("node-%d", nodeCount)
	}

	for i := range 1 {
		seedCounts.Store(i+100, (i+1)*100)
	}

	for i := range 3 {
		nodeInCounts.Store(i+100, []string{})
		nodeOutCounts.Store(i+100, []string{})
	}

	node0, err := n.AddNode(func(ctx context.Context, _ any) (any, error) {
		xtime.SleepWithContext(ctx, time.Second*10)

		num, _ := seedCounts.Load(100)

		if num.(int) > 100 {
			return nil, glow.ErrSeedingDone
		}

		seedCounts.Store(100, num.(int)+1)

		defer func() {
			outCounts, _ := nodeOutCounts.Load(100)
			nodeOutCounts.Store(100, append(outCounts.([]string), strconv.Itoa(num.(int)+1)))
		}()

		return []byte(fmt.Sprintf("%d", num.(int)+1)), nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node1, err := n.AddNode(func(ctx context.Context, in1 any) (any, error) {
		in := in1.([]byte)
		inCounts, _ := nodeInCounts.Load(101)
		nodeInCounts.Store(101, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(101)
			nodeOutCounts.Store(101, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node2, err := n.AddNode(func(ctx context.Context, in1 any) (any, error) {
		in := in1.([]byte)
		inCounts, _ := nodeInCounts.Load(102)
		nodeInCounts.Store(102, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(102)
			nodeOutCounts.Store(102, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	err = n.AddLink(node0, node1)
	if err != nil {
		panic(err)
	}

	err = n.AddLink(node1, node2)
	if err != nil {
		panic(err)
	}
}

func Sub2Network(n *glow.Network) {
	nodeCount := 199
	keygen := func() string {
		nodeCount++
		return fmt.Sprintf("node-%d", nodeCount)
	}

	for i := range 1 {
		seedCounts.Store(i+200, (i+1)*200)
	}

	for i := range 2 {
		nodeInCounts.Store(i+200, []string{})
		nodeOutCounts.Store(i+200, []string{})
	}

	node0, err := n.AddNode(func(ctx context.Context, _ any) (any, error) {
		xtime.SleepWithContext(ctx, time.Second*5)

		num, _ := seedCounts.Load(200)
		seedCounts.Store(200, num.(int)+1)

		defer func() {
			outCounts, _ := nodeOutCounts.Load(200)
			nodeOutCounts.Store(200, append(outCounts.([]string), strconv.Itoa(num.(int)+1)))
		}()

		return []byte(fmt.Sprintf("%d", num.(int)+1)), nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node1, err := n.AddNode(func(ctx context.Context, in1 any) (any, error) {
		in := in1.([]byte)
		inCounts, _ := nodeInCounts.Load(201)
		nodeInCounts.Store(201, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(201)
			nodeOutCounts.Store(201, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	err = n.AddLink(node0, node1)
	if err != nil {
		panic(err)
	}
}

func PrintResults() {
	seedCounts.Range(func(k, v any) bool {
		fmt.Printf("seedCounts[node-%d] Latest = %d\n", k.(int)+1, v)
		return true
	})

	nodeInCounts.Range(func(k, v any) bool {
		fmt.Printf("nodeInCounts [node-%d] = %v (%d)\n", k.(int)+1, v, len(v.([]string)))
		nc, _ := nodeOutCounts.Load(k)
		fmt.Printf("nodeOutCounts[node-%d] = %v (%d)\n", k.(int)+1, nc, len(nc.([]string)))
		return true
	})
}
