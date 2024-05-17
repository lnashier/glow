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
			// Receiver function (method) cannot have type parameters.
			// This is the cleanest way at this moment.
			// https://go.googlesource.com/proposal/+/refs/heads/master/design/43651-type-parameters.md#no-parameterized-methods
			plug.Tokenize(ctx, in.(string), emit)
			return nil
		}, flow.Distributor(), flow.StepKey("tokenizer")).
		Filter(func(in any) bool {
			return strings.HasPrefix(in.(string), "test")
		}, flow.Replicas(5)).
		Count(func(num int) {
			fmt.Println("Count:", num)
		}).
		Draw("bin/network.gv").
		Run(context.Background()).
		Uptime(func(d time.Duration) {
			fmt.Println("Uptime:", d)
		}).
		Draw("bin/network-tally.gv").
		Error()

	if err != nil {
		fmt.Println(err)
	}
}
