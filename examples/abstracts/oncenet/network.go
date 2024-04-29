package oncenet

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	"github.com/lnashier/goarc"
	xtime "github.com/lnashier/goarc/x/time"
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

	node1ID := "node-1"
	seedCounts.Store(node1ID, 0)
	nodeInCounts.Store(node1ID, make([]int, 0))
	nodeOutCounts.Store(node1ID, make([]int, 0))

	_, err := n.AddNode(func(ctx context.Context, _ any) (any, error) {
		num, _ := seedCounts.Load(node1ID)

		if num.(int) > 0 {
			return nil, glow.ErrSeedingDone
		}

		seedCounts.Store(node1ID, num.(int)+1)

		defer func() {
			outCounts, _ := nodeOutCounts.Load(node1ID)
			nodeOutCounts.Store(node1ID, append(outCounts.([]int), num.(int)+1))
		}()

		return num.(int) + 1, nil
	}, glow.Key(node1ID))
	if err != nil {
		panic(err)
	}

	node2ID := "node-2"
	nodeInCounts.Store(node2ID, make([]int, 0))
	nodeOutCounts.Store(node2ID, make([]int, 0))

	_, err = n.AddNode(func(ctx context.Context, in1 any) (any, error) {
		in := in1.(int)
		xtime.SleepWithContext(ctx, time.Second*1)

		inCounts, _ := nodeInCounts.Load(node2ID)
		nodeInCounts.Store(node2ID, append(inCounts.([]int), in))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(node2ID)
			nodeOutCounts.Store(node2ID, append(outCounts.([]int), in))
		}()
		return in, nil
	}, glow.Key(node2ID))
	if err != nil {
		panic(err)
	}

	err = n.AddLink(node1ID, node2ID)
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
		fmt.Printf("nodeInCounts [%s] = %v (%d)\n", k, v, len(v.([]int)))
		nc, _ := nodeOutCounts.Load(k)
		fmt.Printf("nodeOutCounts[%s] = %v (%d)\n", k, nc, len(nc.([]int)))
		return true
	})
}
