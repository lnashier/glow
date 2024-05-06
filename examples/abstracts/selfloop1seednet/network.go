package main

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	"github.com/lnashier/glow/help"
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
	net := Network()
	help.Draw(net, "bin/network.gv")
	goarc.Up(net)
	help.Draw(net, "bin/network-tally.gv")
	PrintResults()
}

func Network() *glow.Network {
	n := glow.New(glow.Verbose())

	node1ID := "node-1"
	seedCounts.Store(node1ID, 0)
	nodeInCounts.Store(node1ID, []string{})
	nodeOutCounts.Store(node1ID, []string{})

	_, err := n.AddNode(glow.NodeFunc(func(ctx context.Context, _ any) (any, error) {
		num, _ := seedCounts.Load(node1ID)

		if num.(int) > 0 {
			return nil, glow.ErrSeedingDone
		}

		xtime.SleepWithContext(ctx, time.Second*5)

		seedCounts.Store(node1ID, num.(int)+1)

		defer func() {
			outCounts, _ := nodeOutCounts.Load(node1ID)
			nodeOutCounts.Store(node1ID, append(outCounts.([]string), strconv.Itoa(num.(int)+1)))
		}()

		return []byte(fmt.Sprintf("%d", num.(int)+1)), nil
	}), glow.Key(node1ID))
	if err != nil {
		panic(err)
	}

	node2ID := "node-2"
	nodeInCounts.Store(node2ID, []string{})
	nodeOutCounts.Store(node2ID, []string{})

	_, err = n.AddNode(glow.NodeFunc(func(ctx context.Context, in1 any) (any, error) {
		in := in1.([]byte)
		// Let's make this node take a nap for a bit, or else it's gonna go all tight loop on us
		// and fill up the log window faster than you can say "Oops!"
		xtime.SleepWithContext(ctx, time.Second*1)

		inCounts, _ := nodeInCounts.Load(node2ID)
		nodeInCounts.Store(node2ID, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(node2ID)
			nodeOutCounts.Store(node2ID, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	}), glow.Key(node2ID))
	if err != nil {
		panic(err)
	}

	// Connect nodes

	err = n.AddLink(node1ID, node2ID)
	if err != nil {
		panic(err)
	}

	// size = count of seeds produced by each seed-node - the length of the loop
	// size = 1 - 0 = 1
	err = n.AddLink(node2ID, node2ID, glow.Size(1))
	if err != nil {
		panic(err)
	}

	return n
}

func PrintResults() {
	seedCounts.Range(func(k, v any) bool {
		fmt.Printf("seedCounts[seed(%s)] = %d\n", k, v)
		return true
	})

	nodeInCounts.Range(func(k, v any) bool {
		fmt.Printf("nodeInCounts [%s] = %v (%d)\n", k, v, len(v.([]string)))
		nc, _ := nodeOutCounts.Load(k)
		fmt.Printf("nodeOutCounts[%s] = %v (%d)\n", k, nc, len(nc.([]string)))
		return true
	})
}
