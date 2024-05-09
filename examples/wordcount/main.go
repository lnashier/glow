package main

import (
	"context"
	"fmt"
	"github.com/lnashier/glow/flow"
	"strings"
)

func main() {
	err := flow.New( /*glow.Verbose()*/ ).
		Read(func(ctx context.Context, emit func(out any)) error {
			return flow.FileReader("test.txt", emit)
		}).
		Map(func(ctx context.Context, in any, emit func(any)) error {
			flow.Tokenize(ctx, in.(string), emit)
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
		Draw("bin/network-tally.gv").
		Error()

	if err != nil {
		fmt.Println(err)
	}
}
