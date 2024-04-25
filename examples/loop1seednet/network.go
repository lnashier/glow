package loop1seednet

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
var node1InCounts []string
var node1OutCounts []string
var node2InCounts []string
var node2OutCounts []string
var node3InCounts []string
var node3OutCounts []string

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
	node0, err := n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		if node0Count > 0 {
			return in, glow.ErrSeedingDone
		}
		xtime.SleepWithContext(ctx, time.Second*5)
		node0Count++

		node0InCounts = append(node0InCounts, string(in))
		defer func() {
			node0OutCounts = append(node0OutCounts, strconv.Itoa(node0Count))
		}()

		return []byte(fmt.Sprintf("%d", node0Count)), nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node1, err := n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		xtime.SleepWithContext(ctx, time.Second*5)

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
		node2InCounts = append(node2InCounts, string(in))
		defer func() {
			node2OutCounts = append(node2OutCounts, string(in))
		}()

		return in, nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	node3, err := n.AddNode(func(ctx context.Context, in []byte) ([]byte, error) {
		node3InCounts = append(node3InCounts, string(in))
		defer func() {
			node3OutCounts = append(node3OutCounts, string(in))
		}()

		return in, nil
	}, glow.KeyFunc(keygen))
	if err != nil {
		panic(err)
	}

	size := 0

	err = n.AddLink(node0, node1, glow.Size(size))
	if err != nil {
		panic(err)
	}

	err = n.AddLink(node1, node2, glow.Size(size))
	if err != nil {
		panic(err)
	}
	err = n.AddLink(node2, node3, glow.Size(size))
	if err != nil {
		panic(err)
	}
	err = n.AddLink(node3, node1, glow.Size(size))
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

	fmt.Printf("node3InCounts %d\n", len(node3InCounts))
	fmt.Println(node3InCounts)
	fmt.Printf("node3OutCounts %d\n", len(node3OutCounts))
	fmt.Println(node3OutCounts)
}
