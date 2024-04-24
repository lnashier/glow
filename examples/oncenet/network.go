package oncenet

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
	n := glow.New(glow.Verbose())

	node1ID := "node-1"
	seedCounts.Store(node1ID, 0)
	nodeInCounts.Store(node1ID, []string{})
	nodeOutCounts.Store(node1ID, []string{})

	_, err := n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		num, _ := seedCounts.Load(node1ID)

		if num.(int) > 0 {
			return nil, glow.ErrSeedingDone
		}

		seedCounts.Store(node1ID, num.(int)+1)

		inCounts, _ := nodeInCounts.Load(node1ID)
		nodeInCounts.Store(node1ID, append(inCounts.([]string), string(in)))

		defer func() {
			outCounts, _ := nodeOutCounts.Load(node1ID)
			nodeOutCounts.Store(node1ID, append(outCounts.([]string), strconv.Itoa(num.(int)+1)))
		}()

		return []byte(fmt.Sprintf("%d", num.(int)+1)), nil
	}, glow.Key(node1ID))
	if err != nil {
		panic(err)
	}

	node2ID := "node-2"
	nodeInCounts.Store(node2ID, []string{})
	nodeOutCounts.Store(node2ID, []string{})

	_, err = n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		xtime.SleepWithContext(ctx, time.Second*1)

		inCounts, _ := nodeInCounts.Load(node2ID)
		nodeInCounts.Store(node2ID, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(node2ID)
			nodeOutCounts.Store(node2ID, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	}, glow.Key(node2ID))
	if err != nil {
		panic(err)
	}

	size := 0

	err = n.AddLink(node1ID, node2ID, size)
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
		fmt.Printf("nodeInCounts [node-%d] = %v\n", k, v)
		nc, _ := nodeOutCounts.Load(k)
		fmt.Printf("nodeOutCounts[node-%d] = %v\n", k, nc)
		return true
	})
}