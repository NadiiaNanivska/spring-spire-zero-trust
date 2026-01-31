package app

import (
	"github.com/spf13/cobra"
)

func AppCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Manage Kubernetes applications",
	}

	cmd.AddCommand(
		appDeployCmd(),
		appPortForwardCmd(),
		appLogsCmd(),
	)

	return cmd
}
