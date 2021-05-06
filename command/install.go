package command

import (
	"log"
	"os"

	"github.com/ray-xu-deltatre/pact-go/install"

	"github.com/spf13/cobra"
)

var path string
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Check required tools",
	Long:  "Checks versions of required Pact CLI tools for used by the library",
	Run: func(cmd *cobra.Command, args []string) {
		setLogLevel(verbose, logLevel)

		// Run the installer
		i := install.NewInstaller()
		var err error
		if err = i.CheckInstallation(); err != nil {
			log.Println("[ERROR] Your Pact CLI installation is out of date, please update to the latest version. Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	installCmd.Flags().StringVarP(&path, "path", "p", "/opt/pact", "Location to install the Pact CLI tools")
	RootCmd.AddCommand(installCmd)
}
