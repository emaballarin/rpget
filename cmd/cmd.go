package cmd

import (
	"github.com/spf13/cobra"

	"github.com/emaballarin/rpget/cmd/multifile"
	"github.com/emaballarin/rpget/cmd/root"
	"github.com/emaballarin/rpget/cmd/version"
)

func GetRootCommand() *cobra.Command {
	rootCMD := root.GetCommand()
	rootCMD.AddCommand(multifile.GetCommand())
	rootCMD.AddCommand(version.VersionCMD)
	return rootCMD
}
