package main

import (
	"os"

	"github.com/emaballarin/rpget/cmd"
	"github.com/emaballarin/rpget/pkg/logging"
)

func main() {
	logging.SetupLogger()
	rootCMD := cmd.GetRootCommand()

	if err := rootCMD.Execute(); err != nil {
		os.Exit(1)
	}
}
