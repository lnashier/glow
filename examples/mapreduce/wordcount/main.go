package main

import (
	"context"
	"fmt"
	"github.com/lnashier/glow/mapreduce"
	"strings"
)

func main() {
	var tokens []any

	err := mapreduce.New().
		Read(func(ctx context.Context, emit func(out any)) error {
			return mapreduce.FileReader("test.txt", emit)
		}).
		Map(func(ctx context.Context, in any, emit func(any)) error {
			mapreduce.Tokenize(ctx, in.(string), emit)
			return nil
		}).
		Filter(func(in any) bool {
			return !strings.HasPrefix(in.(string), "test")
		}).
		Collect(func(ctx context.Context, in any) error {
			tokens = append(tokens, in)
			return nil
		}).
		Draw("bin/network.gv").
		Run().
		Draw("bin/network-tally.gv").
		Error()

	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf("%+v\n", tokens)
}
