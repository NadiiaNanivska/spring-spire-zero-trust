package main

import (
	"fmt"
	"os"

	"wsldev/internal/apps"

	"github.com/spf13/cobra"
)

func appDeployCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "deploy [app-name]",
		Short: "Deploy Kubernetes application",
		Long: `Deploy a Kubernetes application using predefined manifests.

The application must be defined in configs/apps.yaml.
Use --all to deploy all applications.`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			appsMap, err := apps.LoadApps("configs/apps.yaml")
			if err != nil {
				fmt.Println("Failed to load apps config:", err)
				os.Exit(1)
			}

			// Deploy all apps
			if all {
				for name, app := range appsMap {
					fmt.Println("Deploying app:", name)
					if err := apps.Deploy(app); err != nil {
						fmt.Println("Failed to deploy", name, ":", err)
						os.Exit(1)
					}
				}
				return
			}

			// Deploy single app
			if len(args) == 0 {
				fmt.Println("Application name is required (or use --all)")
				os.Exit(1)
			}

			appName := args[0]
			app, ok := appsMap[appName]
			if !ok {
				fmt.Println("Unknown application:", appName)
				os.Exit(1)
			}

			fmt.Println("Deploying app:", appName)
			if err := apps.Deploy(app); err != nil {
				fmt.Println("Deployment failed:", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Deploy all applications")

	return cmd
}
