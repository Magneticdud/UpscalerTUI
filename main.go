package main

import (
	"os"

	"github.com/Magneticdud/UpscalerGUI/internal/cli"
	"github.com/Magneticdud/UpscalerGUI/internal/ui"
)

func main() {
	if len(os.Args) > 1 {
		cli.Run()
		return
	}
	ui.Run()
}
