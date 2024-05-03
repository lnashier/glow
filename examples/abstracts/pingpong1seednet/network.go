package main

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	"github.com/lnashier/glow/help"
	"github.com/lnashier/goarc"
	xtime "github.com/lnashier/goarc/x/time"
	"strconv"
	"time"
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
	goarc.Up(net)
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

	node0Count := 0
	node0, err := n.AddNode(func(ctx context.Context, _ any) (any, error) {
		if node0Count > 0 {
			return nil, glow.ErrSeedingDone
		}
		xtime.SleepWithContext(ctx, time.Second*1)
		node0Count++

		defer func() {
			node0OutCounts = append(node0OutCounts, strconv.Itoa(node0Count))
		}()

		return []byte(fmt.Sprintf("%d", node0Count)), nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node1, err := n.AddNode(func(ctx context.Context, in1 any) (any, error) {
		in := in1.([]byte)
		xtime.SleepWithContext(ctx, time.Second*1)

		node1InCounts = append(node1InCounts, string(in))
		defer func() {
			node1OutCounts = append(node1OutCounts, string(in))
		}()

		return in, nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node2, err := n.AddNode(func(ctx context.Context, in1 any) (any, error) {
		in := in1.([]byte)
		node2InCounts = append(node2InCounts, string(in))
		defer func() {
			node2OutCounts = append(node2OutCounts, string(in))
		}()
		return in, nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	// size = sum (seeds by each seed-node) - length of the loop
	// size = 1 - 1 = 0

	err = n.AddLink(node0, node1)
	if err != nil {
		panic(err)
	}
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
