package main

import (
	"context"
	"fmt"
	"github.com/lnashier/glow/flow"
	"math/rand"
)

func main() {
	err := flow.Sequential( /*glow.Verbose()*/ ).
		Read(func(ctx context.Context, emit func(out any)) error {
			for range 50 {
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
				fmt.Printf("\n%v\n", ints)
			},
			flow.Compare(func(a any, b any) int {
				return a.(int) - b.(int)
			}),
		).
		Draw("bin/network.gv").
		Run().
		Draw("bin/network-tally.gv").
		Error()

	if err != nil {
		fmt.Println("err:", err)
		return
	}
}
