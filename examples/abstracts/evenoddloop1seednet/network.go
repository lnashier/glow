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
	goarc.Up(net)
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

		if num > 2 {
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
		xtime.SleepWithContext(ctx, time.Duration(1)*time.Second)

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

	// Node 1 is acting like a gatekeeper.
	// Node 1 allows odd numbers to pass-through.
	node1ID := keygen()
	nodeInCounts.Store(node1ID, make([]int, 0))
	nodeOutCounts.Store(node1ID, make([]int, 0))
	n.AddNode(glow.EmitFunc(func(ctx context.Context, in1 any, emit func(any)) error {
		in := in1.(int)
		inCounts, _ := nodeInCounts.Load(node1ID)
		nodeInCounts.Store(node1ID, append(inCounts.([]int), in))

		if in%2 != 0 {
			defer func() {
				outCounts, _ := nodeOutCounts.Load(node1ID)
				nodeOutCounts.Store(node1ID, append(outCounts.([]int), in))
			}()
			emit(in)
		}

		return nil
	}), glow.Key(node1ID))

	// Node 2 is acting like a gatekeeper.
	// Node 2 allows even numbers to pass-through.
	node2ID := keygen()
	nodeInCounts.Store(node2ID, make([]int, 0))
	nodeOutCounts.Store(node2ID, make([]int, 0))
	n.AddNode(glow.EmitFunc(func(ctx context.Context, in1 any, emit func(any)) error {
		in := in1.(int)
		inCounts, _ := nodeInCounts.Load(node2ID)
		nodeInCounts.Store(node2ID, append(inCounts.([]int), in))

		if in%2 == 0 {
			defer func() {
				outCounts, _ := nodeOutCounts.Load(node2ID)
				nodeOutCounts.Store(node2ID, append(outCounts.([]int), in))
			}()
			emit(in)
		}

		return nil
	}), glow.Key(node2ID))

	node3ID, _ := addNode(n, keygen)
	node4ID, _ := addNode(n, keygen)
	node5ID, _ := addNode(n, keygen)
	node6ID, _ := addNode(n, keygen)
	node7ID, _ := addNode(n, keygen)
	node8ID, _ := addNode(n, keygen)

	n.AddLink(node0ID, node1ID)
	n.AddLink(node1ID, node3ID)
	n.AddLink(node3ID, node5ID)
	n.AddLink(node5ID, node7ID)
	n.AddLink(node7ID, node3ID)

	n.AddLink(node0ID, node2ID)
	n.AddLink(node2ID, node4ID)
	n.AddLink(node4ID, node6ID)
	n.AddLink(node6ID, node8ID)
	n.AddLink(node8ID, node4ID)

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
