package distributornet

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
	n := glow.New(glow.Verbose(), glow.IgnoreIsolatedNodes())

	node1ID := "node-1"
	seedCounts.Store(node1ID, 0)
	nodeInCounts.Store(node1ID, []string{})
	nodeOutCounts.Store(node1ID, []string{})

	n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		xtime.SleepWithContext(ctx, time.Duration(1)*time.Second)

		num, _ := seedCounts.Load(node1ID)

		/*
			if num.(int) > 10 {
				return nil, glow.ErrSeedingDone
			}
		*/

		seedCounts.Store(node1ID, num.(int)+1)

		inCounts, _ := nodeInCounts.Load(node1ID)
		nodeInCounts.Store(node1ID, append(inCounts.([]string), string(in)))

		defer func() {
			outCounts, _ := nodeOutCounts.Load(node1ID)
			nodeOutCounts.Store(node1ID, append(outCounts.([]string), strconv.Itoa(num.(int)+1)))
		}()

		return []byte(fmt.Sprintf("%d", num.(int)+1)), nil
	}, glow.Key(node1ID), glow.Distributor())

	node2ID := "node-2"
	nodeInCounts.Store(node2ID, []string{})
	nodeOutCounts.Store(node2ID, []string{})

	n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		inCounts, _ := nodeInCounts.Load(node2ID)
		nodeInCounts.Store(node2ID, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(node2ID)
			nodeOutCounts.Store(node2ID, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	}, glow.Key(node2ID))

	node3ID := "node-3"
	nodeInCounts.Store(node3ID, []string{})
	nodeOutCounts.Store(node3ID, []string{})

	n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		inCounts, _ := nodeInCounts.Load(node3ID)
		nodeInCounts.Store(node3ID, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(node3ID)
			nodeOutCounts.Store(node3ID, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	}, glow.Key(node3ID))

	n.AddLink(node1ID, node2ID)
	n.AddLink(node1ID, node3ID)

	return n
}

func PrintResults() {
	seedCounts.Range(func(k, v any) bool {
		fmt.Printf("seedCounts[seed(%s)] = %d\n", k, v)
		return true
	})

	nodeInCounts.Range(func(k, v any) bool {
		fmt.Printf("nodeInCounts [%s] = %v\n", k, v)
		nc, _ := nodeOutCounts.Load(k)
		fmt.Printf("nodeOutCounts[%s] = %v\n", k, nc)
		return true
	})
}
