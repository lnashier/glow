package selfloop1seednet

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
	nodeCount := 0
	n := glow.New(
		func() string {
			nodeCount++
			return fmt.Sprintf("node-%d", nodeCount)
		},
		glow.Verbose(),
	)

	for i := range 1 {
		seedCounts.Store(i+1, 0)
	}

	for i := range 2 {
		nodeInCounts.Store(i+1, []string{})
		nodeOutCounts.Store(i+1, []string{})
	}

	node0, err := n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		xtime.SleepWithContext(ctx, time.Second*5)

		num, _ := seedCounts.Load(1)

		if num.(int) > 2 {
			return nil, glow.ErrSeedingDone
		}

		seedCounts.Store(1, num.(int)+1)

		inCounts, _ := nodeInCounts.Load(1)
		nodeInCounts.Store(1, append(inCounts.([]string), string(in)))

		defer func() {
			outCounts, _ := nodeOutCounts.Load(1)
			nodeOutCounts.Store(1, append(outCounts.([]string), strconv.Itoa(num.(int)+1)))
		}()

		return []byte(fmt.Sprintf("%d", num.(int)+1)), nil
	})
	if err != nil {
		panic(err)
	}

	node1, err := n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		xtime.SleepWithContext(ctx, time.Second*1)

		inCounts, _ := nodeInCounts.Load(2)
		nodeInCounts.Store(2, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(2)
			nodeOutCounts.Store(2, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	})
	if err != nil {
		panic(err)
	}

	// >= seeds - 1
	size := 3

	err = n.AddLink(node0, node1, size)
	if err != nil {
		panic(err)
	}

	err = n.AddLink(node1, node1, size)
	if err != nil {
		panic(err)
	}

	return n
}

func PrintResults() {
	seedCounts.Range(func(k, v any) bool {
		fmt.Printf("seedCounts[seed-%d] = %d\n", k, v)
		return true
	})

	nodeInCounts.Range(func(k, v any) bool {
		fmt.Printf("nodeInCounts [node-%d] = %v\n", k.(int), v)
		nc, _ := nodeOutCounts.Load(k)
		fmt.Printf("nodeOutCounts[node-%d] = %v\n", k.(int), nc)
		return true
	})
}
