package distributornet

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	"github.com/lnashier/goarc"
	xtime "github.com/lnashier/goarc/x/time"
	"os"
	"sync"
	"time"
)

var seedCounts sync.Map
var nodeInCounts sync.Map
var nodeOutCounts sync.Map

func Run() {
	net := Network()

	fmt.Printf("Saving network\n")
	draw(net, "distributornet")
	fmt.Printf("Saving network\n")

	// kick off goroutine to stop the network
	// goarc.Up blocks
	go func() {
		fmt.Println("Preparing to stop network")
		xtime.SleepWithContext(context.Background(), time.Duration(10)*time.Second)
		fmt.Println("Stopping network")
		err := net.Stop()
		if err != nil {
			panic(err)
		}
		fmt.Println("Stopping network")
	}()

	fmt.Printf("Starting network\n")
	goarc.Up(net)
	fmt.Printf("Stopped network\n")

	PrintResults()

	fmt.Printf("Saving network after first run\n")
	draw(net, "distributornet-tally")
	fmt.Printf("Saved network after first run\n")

	// modifications
	modify(net, false)

	fmt.Printf("Saving modified network\n")
	draw(net, "distributornet-modified")
	fmt.Printf("Saved modified network\n")

	// kick off goroutine to stop the network
	// goarc.Up blocks
	go func() {
		fmt.Println("Preparing to stop network to undo modifications")
		xtime.SleepWithContext(context.Background(), time.Duration(10)*time.Second)
		fmt.Println("Stopping network to undo modifications")
		err := net.Stop()
		if err != nil {
			panic(err)
		}
		fmt.Println("Stopped network to undo modifications")
	}()

	fmt.Printf("Starting modified network\n")
	goarc.Up(net)
	fmt.Printf("Stopped modified network\n")

	fmt.Printf("Saving modified network after rerun\n")
	draw(net, "distributornet-modified-tally")
	fmt.Printf("Saved modified network after rerun\n")

	// undo modifications
	modify(net, true)

	fmt.Printf("Saving undone network\n")
	draw(net, "distributornet-undone")
	fmt.Printf("Saved undone network\n")

	fmt.Printf("Starting undone network\n")
	goarc.Up(net)
	fmt.Printf("Stopped undone network\n")

	fmt.Printf("Saving undone network after rerun\n")
	draw(net, "distributornet-undone-tally")
	fmt.Printf("Saved undone network after rerun\n")

	PrintResults()
}

func Network() *glow.Network {
	net := glow.New(glow.Verbose(), glow.IgnoreIsolatedNodes())

	node1ID := "node1"
	addSeed(net, node1ID, glow.Distributor())

	node2ID := "node2"
	addNode(net, node2ID)

	node3ID := "node3"
	addNode(net, node3ID)

	node4ID := "node4"
	addNode(net, node4ID)

	net.AddLink(node1ID, node2ID)
	net.AddLink(node1ID, node3ID)
	net.AddLink(node1ID, node4ID)

	return net
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

func addSeed(net *glow.Network, nodeID string, opt ...glow.NodeOpt) {
	if len(opt) == 0 {
		opt = []glow.NodeOpt{}
	}
	opt = append(opt, glow.Key(nodeID))

	seedCounts.Store(nodeID, 0)
	nodeInCounts.Store(nodeID, make([]int, 0))
	nodeOutCounts.Store(nodeID, make([]int, 0))
	net.AddNode(func(ctx context.Context, _ any) (any, error) {
		xtime.SleepWithContext(ctx, time.Duration(1)*time.Second)

		num1, _ := seedCounts.Load(nodeID)
		num := num1.(int)

		/*
			if num.(int) > 10 {
				return nil, glow.ErrSeedingDone
			}
		*/

		seedCounts.Store(nodeID, num+1)

		defer func() {
			outCounts, _ := nodeOutCounts.Load(nodeID)
			nodeOutCounts.Store(nodeID, append(outCounts.([]int), num+1))
		}()

		return num + 1, nil
	}, opt...)
}

func addNode(net *glow.Network, nodeID string, opt ...glow.NodeOpt) {
	opt = append(opt, glow.Key(nodeID))

	nodeInCounts.Store(nodeID, make([]int, 0))
	nodeOutCounts.Store(nodeID, make([]int, 0))
	net.AddNode(func(ctx context.Context, in1 any) (any, error) {
		in := in1.(int)
		inCounts, _ := nodeInCounts.Load(nodeID)
		nodeInCounts.Store(nodeID, append(inCounts.([]int), in))
		defer func() {
			outCounts, _ := nodeOutCounts.Load(nodeID)
			nodeOutCounts.Store(nodeID, append(outCounts.([]int), in))
		}()
		return in, nil
	}, opt...)
}

func modify(net *glow.Network, undo bool) {
	node1ID := "node1"
	node2ID := "node2"
	node4ID := "node4"

	if undo {
		fmt.Println("Purging network")
		err := net.Purge()
		if err != nil {
			panic(err)
		}
		fmt.Println("Purged network")

		fmt.Printf("Saving purged network\n")
		draw(net, "distributornet-purged")
		fmt.Printf("Saved purged network\n")

		fmt.Printf("Adding node %s %s\n", node2ID)
		addNode(net, node2ID)
		fmt.Printf("Added node %s %s\n", node2ID)

		fmt.Printf("Adding link between %s and %s\n", node1ID, node2ID)
		err = net.AddLink(node1ID, node2ID)
		if err != nil {
			fmt.Printf("Error %v while adding link between %s and %s\n", err, node1ID, node2ID)
			panic(err)
		}
		fmt.Printf("Added link between %s and %s\n", node1ID, node2ID)

		fmt.Printf("Resuming link between %s and %s\n", node1ID, node4ID)
		err = net.ResumeLink(node1ID, node4ID)
		if err != nil {
			fmt.Printf("Error %v while resuming link between %s and %s\n", err, node1ID, node4ID)
			panic(err)
		}
		fmt.Printf("Resumed link between %s and %s\n", node1ID, node4ID)

		return
	}

	fmt.Printf("Removing link between %s and %s\n", node1ID, node2ID)
	err := net.RemoveLink(node1ID, node2ID)
	if err != nil {
		fmt.Printf("Error %v while removing link between %s and %s\n", err, node1ID, node2ID)
		panic(err)
	}
	fmt.Printf("Removed link between %s and %s\n", node1ID, node2ID)

	fmt.Printf("Pausing link between %s and %s\n", node1ID, node4ID)
	err = net.PauseLink(node1ID, node4ID)
	if err != nil {
		fmt.Printf("Error %v while pausing link between %s and %s\n", err, node1ID, node4ID)
		panic(err)
	}
	fmt.Printf("Paused link between %s and %s\n", node1ID, node4ID)
}

func draw(net *glow.Network, name string) {
	data, err := glow.DOT(net)
	if err != nil {
		panic(err)
	}
	if _, err = os.Stat("bin"); os.IsNotExist(err) {
		os.Mkdir("bin", os.FileMode(0755))
	}
	os.WriteFile(fmt.Sprintf("bin/%s.gv", name), data, os.FileMode(0755))
}