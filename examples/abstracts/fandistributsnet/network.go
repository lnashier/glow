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
	n := glow.New(glow.Verbose(), glow.PreventCycles())

	// A seed node working in distributor mode
	node1ID := "node-1"
	addSeed(n, node1ID, false, glow.Distributor())

	// A seed node working in broadcaster mode
	node2ID := "node-2"
	addSeed(n, node2ID, false)

	// A seed node working in distributor and emitter mode
	node3ID := "node-3"
	addSeed(n, node3ID, true, glow.Distributor())

	// A seed node working in broadcaster and emitter mode
	node4ID := "node-4"
	addSeed(n, node4ID, true)

	// A transit node working in distributor mode
	node5ID := "node-5"
	addNode(n, node5ID, false, glow.Distributor())

	// A transit node working in broadcaster mode
	node6ID := "node-6"
	addNode(n, node6ID, false)

	// A transit node working in distributor and emitter mode
	node7ID := "node-7"
	addNode(n, node7ID, true, glow.Distributor())

	// A transit node working in broadcaster and emitter mode
	node8ID := "node-8"
	addNode(n, node8ID, true)

	// A terminal node
	node9ID := "node-9"
	addNode(n, node9ID, false)

	// A terminal node
	node10ID := "node-10"
	addNode(n, node10ID, false)

	n.AddLink(node1ID, node5ID)
	n.AddLink(node1ID, node6ID)

	n.AddLink(node2ID, node5ID)
	n.AddLink(node2ID, node7ID)

	n.AddLink(node3ID, node5ID)
	n.AddLink(node3ID, node6ID)

	n.AddLink(node4ID, node7ID)
	n.AddLink(node4ID, node8ID)

	n.AddLink(node5ID, node9ID)
	n.AddLink(node5ID, node10ID)

	n.AddLink(node6ID, node9ID)
	n.AddLink(node6ID, node10ID)

	n.AddLink(node7ID, node9ID)
	n.AddLink(node7ID, node10ID)

	n.AddLink(node8ID, node9ID)
	n.AddLink(node8ID, node10ID)

	return n
}

func addSeed(net *glow.Network, nodeID string, emitter bool, opt ...glow.NodeOpt) {
	seedCounts.Store(nodeID, 0)
	nodeInCounts.Store(nodeID, make([]int, 0))
	nodeOutCounts.Store(nodeID, make([]int, 0))

	if emitter {
		opt = append(opt, glow.EmitFunc(func(ctx context.Context, _ any, emit func(any)) error {
			for {
				select {
				case <-ctx.Done():
					return nil
				default:
					xtime.SleepWithContext(ctx, time.Duration(1)*time.Second)

					num1, _ := seedCounts.Load(nodeID)
					num := num1.(int) + 1

					seedCounts.Store(nodeID, num)
					outCounts, _ := nodeOutCounts.Load(nodeID)
					nodeOutCounts.Store(nodeID, append(outCounts.([]int), num))

					emit(num)
				}
			}
		}))
	} else {
		opt = append(opt, glow.BasicFunc(func(ctx context.Context, _ any) (any, error) {
			xtime.SleepWithContext(ctx, time.Duration(1)*time.Second)

			num1, _ := seedCounts.Load(nodeID)
			num := num1.(int) + 1

			seedCounts.Store(nodeID, num)

			defer func() {
				outCounts, _ := nodeOutCounts.Load(nodeID)
				nodeOutCounts.Store(nodeID, append(outCounts.([]int), num))
			}()
			return num, nil
		}))
	}

	net.AddNode(append(opt, glow.Key(nodeID))...)
}

func addNode(net *glow.Network, nodeID string, emitter bool, opt ...glow.NodeOpt) {
	nodeInCounts.Store(nodeID, make([]int, 0))
	nodeOutCounts.Store(nodeID, make([]int, 0))

	if emitter {
		opt = append(opt, glow.EmitFunc(func(ctx context.Context, in1 any, emit func(any)) error {
			in := in1.(int)
			inCounts, _ := nodeInCounts.Load(nodeID)
			nodeInCounts.Store(nodeID, append(inCounts.([]int), in))

			repeat := 0

			for {
				select {
				case <-ctx.Done():
					return nil
				default:
					emit(in)
					outCounts, _ := nodeOutCounts.Load(nodeID)
					nodeOutCounts.Store(nodeID, append(outCounts.([]int), in))
					repeat++
					if repeat == 2 {
						return nil
					}
				}
			}
		}))
	} else {
		opt = append(opt, glow.BasicFunc(func(ctx context.Context, in1 any) (any, error) {
			in := in1.(int)
			inCounts, _ := nodeInCounts.Load(nodeID)
			nodeInCounts.Store(nodeID, append(inCounts.([]int), in))
			defer func() {
				outCounts, _ := nodeOutCounts.Load(nodeID)
				nodeOutCounts.Store(nodeID, append(outCounts.([]int), in))
			}()
			return in, nil
		}))
	}

	net.AddNode(append(opt, glow.Key(nodeID))...)
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
