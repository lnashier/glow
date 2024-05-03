package help

import (
	"github.com/lnashier/glow"
	"os"
)

// Draw generates a DOT description of the glow.Network and saves it to the specified path.
func Draw(net *glow.Network, name string) error {
	data, err := glow.DOT(net)
	if err != nil {
		return err
	}
	return os.WriteFile(name, data, os.FileMode(0755))
}
