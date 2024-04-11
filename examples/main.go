package main

import (
	"context"
	"examples/badnet"
	"examples/fan1seednet"
	"examples/fannet"
	"examples/loop1seednet"
	"examples/onewaynet"
	"examples/pingpong1seednet"
	"examples/pingpong2seednet"
	"examples/subwaynet"
	"fmt"
	"github.com/lnashier/glow"
	"github.com/lnashier/goarc"
	goarccli "github.com/lnashier/goarc/cli"
	"os"
)

func main() {
	goarc.Up(goarccli.NewService(
		goarccli.ServiceName("Examples"),
		goarccli.App(func(svc *goarccli.Service) error {
			svc.Register("draw", func(ctx context.Context, args []string) error {
				net, _ := newNet(args[0])
				data, err := glow.DOT(net)
				if err != nil {
					fmt.Println(err)
					return err
				}

				if _, err = os.Stat("bin"); os.IsNotExist(err) {
					os.Mkdir("bin", os.FileMode(0755))
				}
				return os.WriteFile(fmt.Sprintf("bin/%s.gv", args[0]), data, os.FileMode(0755))
			})

			svc.Register("up", func(ctx context.Context, args []string) error {
				net, fn := newNet(args[0])
				goarc.Up(net)
				fn()
				return nil
			})
			return nil
		}),
	))
}

func newNet(name string) (*glow.Network, func()) {
	switch name {
	case "badnet":
		return badnet.Network(), badnet.PrintResults
	case "fan1seednet":
		return fan1seednet.Network(), fan1seednet.PrintResults
	case "fannet":
		return fannet.Network(), fannet.PrintResults
	case "loop1seednet":
		return loop1seednet.Network(), loop1seednet.PrintResults
	case "onewaynet":
		return onewaynet.Network(), onewaynet.PrintResults
	case "pingpong1seednet":
		return pingpong1seednet.Network(), pingpong1seednet.PrintResults
	case "pingpong2seednet":
		return pingpong2seednet.Network(), pingpong2seednet.PrintResults
	case "subwaynet":
		return subwaynet.Network(), subwaynet.PrintResults
	default:
		return nil, nil
	}
}
