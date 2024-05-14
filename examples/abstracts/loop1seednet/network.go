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

func addSeed(net *glow.Network, keygen func() string, opt ...glow.NodeOpt) (string, error) {
	nodeID := keygen()
	seedCounts.Store(nodeID, 0)
	nodeInCounts.Store(nodeID, make([]int, 0))
	nodeOutCounts.Store(nodeID, make([]int, 0))

	opt = append(opt, glow.NodeFunc(func(ctx context.Context, _ any) (any, error) {
		num1, _ := seedCounts.Load(nodeID)
		num := num1.(int) + 1

		if num > 1 {
			return nil, glow.ErrSeedingDone
		}

		seedCounts.Store(nodeID, num)

		defer func() {
			outCounts, _ := nodeOutCounts.Load(nodeID)
			nodeOutCounts.Store(nodeID, append(outCounts.([]int), num))
		}()
		return num, nil
	}))

	return net.AddNode(append(opt, glow.Key(nodeID))...)
}

func addNode(net *glow.Network, keygen func() string, opt ...glow.NodeOpt) (string, error) {
	nodeID := keygen()
	nodeInCounts.Store(nodeID, make([]int, 0))
	nodeOutCounts.Store(nodeID, make([]int, 0))

	opt = append(opt, glow.NodeFunc(func(ctx context.Context, in1 any) (any, error) {
		xtime.SleepWithContext(ctx, time.Second*1)
		in := in1.(int)
		inCounts, _ := nodeInCounts.Load(nodeID)
		nodeInCounts.Store(nodeID, append(inCounts.([]int), in))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(nodeID)
			nodeOutCounts.Store(nodeID, append(outCounts.([]int), in))
		}()
		return in, nil
	}))

	return net.AddNode(append(opt, glow.Key(nodeID))...)
}

func Network() *glow.Network {
	n := glow.New(glow.Verbose())

	nodeCount := -1
	keygen := func() string {
		nodeCount++
		return fmt.Sprintf("node-%d", nodeCount)
	}

	node0ID, _ := addSeed(n, keygen)
	node1ID, _ := addNode(n, keygen)
	node2ID, _ := addNode(n, keygen)
	node3ID, _ := addNode(n, keygen)

	n.AddLink(node0ID, node1ID)
	n.AddLink(node1ID, node2ID)
	n.AddLink(node2ID, node3ID)
	n.AddLink(node3ID, node1ID)

	return n
}

func PrintResults() {
	seedCounts.Range(func(k, v any) bool {
		fmt.Printf("seedCounts[seed(%s)] = %d\n", k, v)
		return true
	})

	nodeInCounts.Range(func(k, v any) bool {
		fmt.Printf("nodeInCounts [%s] = %v (%d)\n", k, v, len(v.([]int)))
		nc, _ := nodeOutCounts.Load(k)
		fmt.Printf("nodeOutCounts[%s] = %v (%d)\n", k, nc, len(nc.([]int)))
		return true
	})
}
