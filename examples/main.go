package main

import (
	"context"
	"examples/badnet"
	"examples/distributornet"
	"examples/fan1seednet"
	"examples/fandistributsnet"
	"examples/fannet"
	"examples/loop1seednet"
	"examples/oncenet"
	"examples/onewaynet"
	"examples/pingpong1seednet"
	"examples/pingpong2seednet"
	"examples/pingpongnet"
	"examples/selfloop1seednet"
	"examples/selfloopnet"
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
				if net != nil {
					data, err := glow.DOT(net)
					if err != nil {
						fmt.Println(err)
						return err
					}

					if _, err = os.Stat("bin"); os.IsNotExist(err) {
						os.Mkdir("bin", os.FileMode(0755))
					}
					return os.WriteFile(fmt.Sprintf("bin/%s.gv", args[0]), data, os.FileMode(0755))
				}
				return nil
			})

			svc.Register("up", func(ctx context.Context, args []string) error {
				net, fn := newNet(args[0])
				if net != nil {
					goarc.Up(net)
					if fn != nil {
						fn()
					}
				}
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
	case "distributornet":
		return distributornet.Network(), distributornet.PrintResults
	case "fan1seednet":
		return fan1seednet.Network(), fan1seednet.PrintResults
	case "fandistributsnet":
		return fandistributsnet.Network(), fandistributsnet.PrintResults
	case "fannet":
		return fannet.Network(), fannet.PrintResults
	case "loop1seednet":
		return loop1seednet.Network(), loop1seednet.PrintResults
	case "oncenet":
		return oncenet.Network(), oncenet.PrintResults
	case "onewaynet":
		return onewaynet.Network(), onewaynet.PrintResults
	case "pingpong1seednet":
		return pingpong1seednet.Network(), pingpong1seednet.PrintResults
	case "pingpong2seednet":
		return pingpong2seednet.Network(), pingpong2seednet.PrintResults
	case "pingpongnet":
		return pingpongnet.Network(), pingpongnet.PrintResults
	case "selfloop1seednet":
		return selfloop1seednet.Network(), selfloop1seednet.PrintResults
	case "selfloopnet":
		return selfloopnet.Network(), selfloopnet.PrintResults
	case "subwaynet":
		return subwaynet.Network(), subwaynet.PrintResults
	default:
		return nil, nil
	}
}
