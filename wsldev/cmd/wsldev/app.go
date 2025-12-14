package main

import (
	"github.com/spf13/cobra"
)

func appCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Manage Kubernetes applications",
	}

	cmd.AddCommand(
		appDeployCmd(),
		//appDeleteCmd(),
		//appRestartCmd(),
		//appStatusCmd(),
		//appLogsCmd(),
	)

	return cmd
}
