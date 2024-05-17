package main

import (
	"context"
	"fmt"
	"github.com/lnashier/glow/flow"
	"github.com/lnashier/goarc"
	"math/rand"
	"time"
)

func main() {
	seq := flow.Sequential( /*glow.Verbose()*/ )

	// Add a hook to listen to exit signal
	goarc.Up(goarc.ServiceFunc(func(starting bool) error {
		if !starting {
			seq.Stop()
		}
		return nil
	}))

	err := seq.
		Read(func(ctx context.Context, emit func(out any)) error {
			for range 50 {
				//time.Sleep(1 * time.Second)
				select {
				case <-ctx.Done():
					return nil
				default:
					emit(rand.Intn(101))
				}
			}
			return nil
		}).
		Peek(func(data any) {
			fmt.Printf("%v ", data)
		}).
		Collect(
			func(ints []any) {
				fmt.Printf("\n%v (%d)\n", ints, len(ints))
			},
			func(a any, b any) int {
				return a.(int) - b.(int)
			},
		).
		Draw("bin/network.gv").
		Run(context.Background()).
		Uptime(func(d time.Duration) {
			fmt.Println(d)
		}).
		Draw("bin/network-tally.gv").
		Error()

	if err != nil {
		fmt.Println("err:", err)
		return
	}
}
