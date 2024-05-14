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

	n := glow.New(glow.Verbose(), glow.PreventCycles())

	node0Count := 0
	node0, err := n.AddNode(glow.NodeFunc(func(ctx context.Context, _ any) (any, error) {
		xtime.SleepWithContext(ctx, time.Second*1)

		node0Count++

		defer func() {
			node0OutCounts = append(node0OutCounts, strconv.Itoa(node0Count))
		}()

		return []byte(fmt.Sprintf("%d", node0Count)), nil
	}), glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node1, err := n.AddNode(glow.NodeFunc(func(ctx context.Context, in1 any) (any, error) {
		in := in1.([]byte)
		xtime.SleepWithContext(ctx, time.Second*1)

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
		xtime.SleepWithContext(ctx, time.Second*1)

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

	err = n.AddLink(node1, node2)
	if err != nil {
		panic(err)
	}

	return n
}

func PrintResults() {
	fmt.Printf("node0InCounts %d\n", len(node0InCounts))
	fmt.Println(node0InCounts)
	fmt.Printf("node0OutCounts %d\n", len(node0OutCounts))
	fmt.Println(node0OutCounts)

	fmt.Printf("node1InCounts %d\n", len(node1InCounts))
	fmt.Println(node1InCounts)
	fmt.Printf("node1OutCounts %d\n", len(node1OutCounts))
	fmt.Println(node1OutCounts)

	fmt.Printf("node2InCounts %d\n", len(node2InCounts))
	fmt.Println(node2InCounts)
	fmt.Printf("node2OutCounts %d\n", len(node2OutCounts))
	fmt.Println(node2OutCounts)
}
