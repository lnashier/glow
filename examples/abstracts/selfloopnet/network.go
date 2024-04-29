package selfloopnet

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
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
	goarc.Up(Network())
	PrintResults()
}

func Network() *glow.Network {
	n := glow.New(glow.Verbose(), glow.StopGracetime(time.Duration(5)*time.Second))

	node1ID := "node-1"
	seedCounts.Store(node1ID, 0)
	nodeInCounts.Store(node1ID, []string{})
	nodeOutCounts.Store(node1ID, []string{})

	seed := func(ctx context.Context, _ any) (any, error) {
		xtime.SleepWithContext(ctx, time.Duration(1)*time.Second)

		num, _ := seedCounts.Load(node1ID)

		seedCounts.Store(node1ID, num.(int)+1)

		defer func() {
			outCounts, _ := nodeOutCounts.Load(node1ID)
			nodeOutCounts.Store(node1ID, append(outCounts.([]string), strconv.Itoa(num.(int)+1)))
		}()

		return []byte(fmt.Sprintf("%d", num.(int)+1)), nil
	}

	seedingDone := false

	_, err := n.AddNode(func(ctx context.Context, in1 any) (any, error) {
		if !seedingDone {
			defer func() {
				seedingDone = true
			}()
			return seed(ctx, in1)
		}

		in := in1.([]byte)

		xtime.SleepWithContext(ctx, time.Second*2)

		inCounts, _ := nodeInCounts.Load(node1ID)
		nodeInCounts.Store(node1ID, append(inCounts.([]string), string(in)))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(node1ID)
			nodeOutCounts.Store(node1ID, append(outCounts.([]string), string(in)))
		}()
		return in, nil
	}, glow.Key(node1ID))
	if err != nil {
		panic(err)
	}

	err = n.AddLink(node1ID, node1ID)
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
