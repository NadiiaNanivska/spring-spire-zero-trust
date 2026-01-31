package spire

import (
	"github.com/spf13/cobra"
	"wsldev/internal/spire"
)

func SpireCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spire",
		Short: "SPIRE management",
	}

	cmd.AddCommand(spireDeployCmd())
	cmd.AddCommand(spireEntryCmd())

	return cmd
}

func spireDeployCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deploy",
		Short: "Deploy SPIRE to Kubernetes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return spire.Deploy()
		},
	}
}
