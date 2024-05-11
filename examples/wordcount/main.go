package main

import (
	"context"
	"fmt"
	"github.com/lnashier/glow/flow"
	"github.com/lnashier/glow/flow/plug"
	"strings"
	"time"
)

func main() {
	err := flow.Sequential( /*glow.Verbose()*/ ).
		Read(func(ctx context.Context, emit func(out any)) error {
			return plug.ReadFile("test.txt", emit)
		}).
		Map(func(ctx context.Context, in any, emit func(any)) error {
			plug.Tokenize(ctx, in.(string), emit)
			return nil
		}).
		Filter(func(in any) bool {
			return strings.HasPrefix(in.(string), "test")
		}).
		Count(func(num int) {
			fmt.Println("count:", num)
		}).
		Draw("bin/network.gv").
		Run().
		Uptime(func(d time.Duration) {
			fmt.Println(d)
		}).
		Draw("bin/network-tally.gv").
		Error()

	if err != nil {
		fmt.Println(err)
	}
}
