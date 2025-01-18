package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/emaballarin/rpget/pkg/version"
)

const VersionCMDName = "version"

var VersionCMD = &cobra.Command{
	Use:   VersionCMDName,
	Short: "print version and build information",
	Long:  "Print the version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("rpget Version %s - Build Time %s\n", version.GetVersion(), version.BuildTime)
	},
}
