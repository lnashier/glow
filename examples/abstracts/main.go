package main

import (
	"context"
	"examples/distributornet"
	"examples/fan1seednet"
	"examples/fandistributsnet"
	"examples/fannet"
	"examples/loop1seednet"
	"examples/oncenet"
	"examples/onewaynet"
	"examples/pingpong1seednet"
	"examples/pingpong2seednet"
	"examples/pingponginfiniteseednet"
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
					data, err := glow.DOT(net())
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
				_, up := newNet(args[0])
				up()
				return nil
			})

			return nil
		}),
	))
}

func newNet(name string) (func() *glow.Network, func()) {
	switch name {
	case "pingponginfiniteseednet":
		return pingponginfiniteseednet.Network, pingponginfiniteseednet.Run
	case "distributornet":
		return distributornet.Network, distributornet.Run
	case "fan1seednet":
		return fan1seednet.Network, fan1seednet.Run
	case "fandistributsnet":
		return fandistributsnet.Network, fandistributsnet.Run
	case "fannet":
		return fannet.Network, fannet.Run
	case "loop1seednet":
		return loop1seednet.Network, loop1seednet.Run
	case "oncenet":
		return oncenet.Network, oncenet.Run
	case "onewaynet":
		return onewaynet.Network, onewaynet.Run
	case "pingpong1seednet":
		return pingpong1seednet.Network, pingpong1seednet.Run
	case "pingpong2seednet":
		return pingpong2seednet.Network, pingpong2seednet.Run
	case "pingpongnet":
		return pingpongnet.Network, pingpongnet.Run
	case "selfloop1seednet":
		return selfloop1seednet.Network, selfloop1seednet.Run
	case "selfloopnet":
		return selfloopnet.Network, selfloopnet.Run
	case "subwaynet":
		return subwaynet.Network, subwaynet.Run
	default:
		return nil, nil
	}
}
