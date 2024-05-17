package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/lnashier/glow/flow"
	"strings"
	"time"
)

func main() {
	found := false

	err := flow.Sequential().
		Read(func(ctx context.Context, emit func(any)) error {
			for !found {
				select {
				case <-ctx.Done():
					return nil
				default:
					randomBytes := make([]byte, 16)
					_, err := rand.Read(randomBytes)
					if err != nil {
						return err
					}
					hasher := sha256.New()
					_, err = hasher.Write(randomBytes)
					if err != nil {
						return err
					}
					emit(hex.EncodeToString(hasher.Sum(nil)))
				}
			}
			return nil
		}, flow.StepKey("generator"), flow.Replicas(2), flow.Distributor()).
		Filter(func(in any) bool {
			return strings.HasPrefix(in.(string), "000")
		}, flow.StepKey("filter"), flow.Replicas(4)).
		Peek(func(in any) {
			found = true
			fmt.Println(in.(string))
		}, flow.StepKey("print")).
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
