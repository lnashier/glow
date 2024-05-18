package main

import (
	"context"
	"fmt"
	"github.com/lnashier/glow"
	"github.com/lnashier/glow/flow"
	"github.com/lnashier/goarc"
	xtime "github.com/lnashier/goarc/x/time"
	"sync"
	"time"
)

var seedCounts sync.Map
var nodeInCounts sync.Map
var nodeOutCounts sync.Map

func Run() {
	step0ID := "seed"
	seedCounts.Store(step0ID, 0)
	nodeInCounts.Store(step0ID, make([]int, 0))
	nodeOutCounts.Store(step0ID, make([]int, 0))

	step1ID := "step1"
	nodeInCounts.Store(step1ID, make([]int, 0))
	nodeOutCounts.Store(step1ID, make([]int, 0))

	step2ID := "step2"
	nodeInCounts.Store(step2ID, make([]int, 0))
	nodeOutCounts.Store(step2ID, make([]int, 0))

	net := flow.New(glow.Verbose())

	// Add a hook to listen to exit signal
	goarc.Up(goarc.ServiceFunc(func(starting bool) error {
		if !starting {
			net.Stop()
		}
		return nil
	}))

	err := net.
		Step(
			flow.StepKey(step0ID),
			flow.Read(func(ctx context.Context, emit func(any)) error {
				outCounts, _ := nodeOutCounts.Load(step0ID)
				for i := range 1 {
					num := i + 1
					emit(num)
					seedCounts.Store(step0ID, num)
					nodeOutCounts.Store(step0ID, append(outCounts.([]int), num))
				}
				return nil
			}),
		).
		Step(
			flow.StepKey(step1ID),
			flow.Map(func(ctx context.Context, in1 any, emit func(any)) error {
				in := in1.(int)
				xtime.SleepWithContext(ctx, time.Second*1)
				inCounts, _ := nodeInCounts.Load(step1ID)
				nodeInCounts.Store(step1ID, append(inCounts.([]int), in))
				emit(in)
				outCounts, _ := nodeOutCounts.Load(step1ID)
				nodeOutCounts.Store(step1ID, append(outCounts.([]int), in))
				return nil
			}),
			flow.Connection(step0ID, step2ID),
		).
		Step(
			flow.StepKey(step2ID),
			flow.Map(func(ctx context.Context, in1 any, emit func(any)) error {
				in := in1.(int)
				inCounts, _ := nodeInCounts.Load(step2ID)
				nodeInCounts.Store(step2ID, append(inCounts.([]int), in))
				emit(in)
				outCounts, _ := nodeOutCounts.Load(step2ID)
				nodeOutCounts.Store(step2ID, append(outCounts.([]int), in))
				return nil
			}),
			flow.Connection(step1ID),
		).
		Draw("bin/network.gv").
		Run(context.Background()).
		Draw("bin/network-tally.gv").
		Error()

	if err != nil {
		fmt.Println("Err: ", err)
		return
	}

	PrintResults()
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
