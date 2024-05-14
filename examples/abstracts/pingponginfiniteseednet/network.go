package main

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	"github.com/lnashier/glow/help"
	"github.com/lnashier/goarc"
	"strconv"
)

var node0InCounts []string
var node0OutCounts []string
var node1InCounts []string
var node1OutCounts []string
var node2InCounts []string
var node2OutCounts []string

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
	nodeCount := -1
	keygen := func() string {
		nodeCount++
		return fmt.Sprintf("node-%d", nodeCount)
	}

	n := glow.New(glow.Verbose())

	node0count := 0
	node0, err := n.AddNode(glow.NodeFunc(func(ctx context.Context, _ any) (any, error) {
		// un/comment to play around, change time
		//xtime.SleepWithContext(ctx, time.Second*5)
		node0count++

		node0InCounts = append(node0InCounts, "")
		defer func() {
			node0OutCounts = append(node0OutCounts, strconv.Itoa(node0count))
		}()

		return []byte(fmt.Sprintf("%d", node0count)), nil
	}), glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node1, err := n.AddNode(glow.NodeFunc(func(ctx context.Context, in1 any) (any, error) {
		in := in1.([]byte)
		// un/comment to play around, change time
		//xtime.SleepWithContext(ctx, time.Second*1)

		node1InCounts = append(node1InCounts, string(in))
		defer func() {
			node1OutCounts = append(node1OutCounts, string(in))
		}()

		return in, nil
	}), glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node2, err := n.AddNode(glow.NodeFunc(func(ctx context.Context, in1 any) (any, error) {
		in := in1.([]byte)
		// un/comment to play around, change time
		//xtime.SleepWithContext(ctx, time.Second*1)

		node2InCounts = append(node2InCounts, string(in))
		defer func() {
			node2OutCounts = append(node2OutCounts, string(in))
		}()

		return in, nil
	}), glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	err = n.AddLink(node0, node1)
	if err != nil {
		panic(err)
	}

	// size = count of seeds produced by each seed-node - the length of the loop
	// size = inf - 1 = inf
	// one of the link needs to have proper size
	// Depending on how often seed-node is producing seeds, eventually system will come to halt

	//err = n.AddLink(node1, node2, glow.Size(10))
	err = n.AddLink(node1, node2)
	if err != nil {
		panic(err)
	}
	err = n.AddLink(node2, node1)
	if err != nil {
		panic(err)
	}

	return n
}

func PrintResults() {
	fmt.Printf("node0InCounts = %v (%d)\n", node0InCounts, len(node0InCounts))
	fmt.Printf("node0OutCounts = %v (%d)\n", node0OutCounts, len(node0OutCounts))
	fmt.Printf("node1InCounts = %v (%d)\n", node1InCounts, len(node1InCounts))
	fmt.Printf("node1OutCounts = %v (%d)\n", node1OutCounts, len(node1OutCounts))
	fmt.Printf("node2InCounts = %v (%d)\n", node2InCounts, len(node2InCounts))
	fmt.Printf("node2OutCounts = %v (%d)\n", node2OutCounts, len(node2OutCounts))
}
