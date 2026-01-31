package app

import (
	"fmt"
	"os"

	"wsldev/internal/apps"

	"github.com/spf13/cobra"
)

func appDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [app-name]",
		Short: "Deploy Kubernetes application",
		Long:  `Deploy a Kubernetes application using predefined manifests.`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			for _, appName := range args {
				fmt.Println("Deploying:", appName)

				if err := apps.DeployByName(appName); err != nil {
					fmt.Println("Deploy failed:", err)
					os.Exit(1)
				}
			}
		},
	}

	return cmd
}
