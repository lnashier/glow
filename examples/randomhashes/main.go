package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/lnashier/glow/flow"
	"github.com/lnashier/goarc"
	"strings"
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

	found := false

	err := seq.
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
		}, flow.StepKey("generator"), flow.Replicas(10), flow.Distributor()).
		Filter(func(in any) bool {
			if found {
				return false
			}
			found = strings.HasPrefix(in.(string), "000000")
			return found
		}, flow.StepKey("filter"), flow.Replicas(4)).
		Collect(
			func(hashes []any) {
				fmt.Printf("%v (%d)\n", hashes, len(hashes))
			},
			nil,
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
