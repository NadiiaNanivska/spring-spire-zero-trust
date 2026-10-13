package app

import (
	"fmt"

	"wsldev/internal/apps"

	"github.com/spf13/cobra"
)

func appDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [app-name]",
		Short: "Deploy Kubernetes application",
		Long:  `Deploy a Kubernetes application using predefined manifests.`,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, appName := range args {
				fmt.Println("Deploying:", appName)

				if err := apps.DeployByName(appName); err != nil {
					return fmt.Errorf("deploy %s: %w", appName, err)
				}
			}

			if err := apps.SyncJVMHashes(args); err != nil {
				return fmt.Errorf("sync JVM provenance configuration: %w", err)
			}
			if err := apps.RegisterJVMWorkloads(args); err != nil {
				return fmt.Errorf("register JVM workloads: %w", err)
			}
			return nil
		},
	}

	return cmd
}
