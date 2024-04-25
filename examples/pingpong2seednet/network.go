package pingpong2seednet

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	"github.com/lnashier/goarc"
	xtime "github.com/lnashier/goarc/x/time"
	"strconv"
	"time"
)

var node0InCounts []string
var node0OutCounts []string
var node00InCounts []string
var node00OutCounts []string
var node1InCounts []string
var node1OutCounts []string
var node2InCounts []string
var node2OutCounts []string

func Run() {
	goarc.Up(Network())
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
	node0, err := n.AddNode(func(ctx context.Context, _ []byte) ([]byte, error) {
		if node0Count > 0 {
			return nil, glow.ErrSeedingDone
		}

		xtime.SleepWithContext(ctx, time.Second*5)
		node0Count++

		node0InCounts = append(node0InCounts, "")
		defer func() {
			node0OutCounts = append(node0OutCounts, strconv.Itoa(node0Count))
		}()

		return []byte(fmt.Sprintf("%d", node0Count)), nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node00count := 10
	node00, err := n.AddNode(func(ctx context.Context, _ []byte) ([]byte, error) {
		if node00count > 10 {
			return nil, glow.ErrSeedingDone
		}

		xtime.SleepWithContext(ctx, time.Second*5)
		node00count++

		node00InCounts = append(node00InCounts, "")
		defer func() {
			node00OutCounts = append(node00OutCounts, strconv.Itoa(node00count))
		}()

		return []byte(fmt.Sprintf("%d", node00count)), nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node1, err := n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		node1InCounts = append(node1InCounts, string(in))
		defer func() {
			node1OutCounts = append(node1OutCounts, string(in))
		}()

		return in, nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node2, err := n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		xtime.SleepWithContext(ctx, time.Second*5)

		node2InCounts = append(node2InCounts, string(in))
		defer func() {
			node2OutCounts = append(node2OutCounts, string(in))
		}()

		return in, nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	err = n.AddLink(node0, node1)
	if err != nil {
		panic(err)
	}
	err = n.AddLink(node00, node1)
	if err != nil {
		panic(err)
	}

	// size = count of seeds produced by each seed-node - the length of the loop
	// size = 1 + 1 - 1 = 1
	// one of the link needs to have proper size

	err = n.AddLink(node1, node2, glow.Size(1))
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
	fmt.Printf("node00InCounts = %v (%d)\n", node00InCounts, len(node00InCounts))
	fmt.Printf("node00OutCounts = %v (%d)\n", node00OutCounts, len(node00OutCounts))
	fmt.Printf("node1InCounts = %v (%d)\n", node1InCounts, len(node1InCounts))
	fmt.Printf("node1OutCounts = %v (%d)\n", node1OutCounts, len(node1OutCounts))
	fmt.Printf("node2InCounts = %v (%d)\n", node2InCounts, len(node2InCounts))
	fmt.Printf("node2OutCounts = %v (%d)\n", node2OutCounts, len(node2OutCounts))
}
