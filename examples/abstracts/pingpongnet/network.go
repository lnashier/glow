package main

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	"github.com/lnashier/glow/help"
	"github.com/lnashier/goarc"
	xtime "github.com/lnashier/goarc/x/time"
	"sync"
	"time"
)

var seedCounts sync.Map
var nodeInCounts sync.Map
var nodeOutCounts sync.Map

func Run() {
	net := Network()
	help.Draw(net, "bin/network.gv")
	goarc.Up(goarc.ServiceFunc(func(starting bool) error {
		if starting {
			return net.Start(context.Background())
		}
		return net.Stop()
	}))
	help.Draw(net, "bin/network-tally.gv")
	PrintResults()
}

func Network() *glow.Network {
	nodeCount := 0
	keygen := func() string {
		nodeCount++
		return fmt.Sprintf("node-%d", nodeCount)
	}

	n := glow.New(glow.Verbose())

	for i := range 1 {
		nodeInCounts.Store(i+1, []string{})
		nodeOutCounts.Store(i+1, []string{})
	}

	node1, err := n.AddNode(glow.BasicFunc(func(ctx context.Context, in1 any) (any, error) {
		in := in1.([]byte)
		xtime.SleepWithContext(ctx, time.Second*1)

		id := 1

		inCounts, _ := nodeInCounts.Load(id)
		nodeInCounts.Store(id, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(id)
			nodeOutCounts.Store(id, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	}), glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node2, err := n.AddNode(glow.BasicFunc(func(ctx context.Context, in1 any) (any, error) {
		in := in1.([]byte)
		xtime.SleepWithContext(ctx, time.Second*1)

		id := 2

		inCounts, _ := nodeInCounts.Load(id)
		nodeInCounts.Store(id, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(id)
			nodeOutCounts.Store(id, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	}), glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	err = n.AddLink(node1, node2, glow.Size(1))
	if err != nil {
		panic(err)
	}

	err = n.AddLink(node2, node1, glow.Size(1))
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
		fmt.Printf("nodeInCounts [node-%d] = %v (%d)\n", k.(int)+1, v, len(v.([]string)))
		nc, _ := nodeOutCounts.Load(k)
		fmt.Printf("nodeOutCounts[node-%d] = %v (%d)\n", k.(int)+1, nc, len(nc.([]string)))
		return true
	})
}
