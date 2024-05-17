package main

import (
	"context"
	"fmt"
	"github.com/lnashier/glow/flow"
	"math/rand"
	"time"
)

func main() {
	err := flow.New().
		Step(
			flow.StepKey("generator"),
			flow.Distributor(),
			flow.Read(func(ctx context.Context, emit func(out any)) error {
				for range 50 {
					select {
					case <-ctx.Done():
						return nil
					default:
						emit(rand.Intn(1001))
					}
				}
				return nil
			}),
		).
		Step(
			flow.StepKey("filter"),
			flow.Replicas(4),
			flow.Filter(func(in any) bool {
				return in.(int)%2 == 0
			}),
			flow.Connection("generator"),
		).
		Step(
			flow.StepKey("collector"),
			flow.Collect(
				func(ints []any) {
					fmt.Printf("%v (%d)\n", ints, len(ints))
				},
				func(a any, b any) int {
					return a.(int) - b.(int)
				},
			),
			flow.Connection("filter"),
		).
		Draw("bin/network.gv").
		Run(context.Background()).
		Uptime(func(d time.Duration) {
			fmt.Println("Uptime", d)
		}).
		Draw("bin/network-tally.gv").
		Error()

	if err != nil {
		fmt.Println("err:", err)
		return
	}
}
