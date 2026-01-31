package spire

import (
	"github.com/spf13/cobra"
	"wsldev/internal/spire"
)

func spireEntryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entry",
		Short: "SPIRE entry management",
	}

	cmd.AddCommand(spireEntryCreateCmd())
	cmd.AddCommand(spireEntryShowCmd())

	return cmd
}

func spireEntryCreateCmd() *cobra.Command {
	var spiffeID, parentID, namespace, sa string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create SPIRE entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			return spire.EntryCreate(spiffeID, parentID, namespace, sa)
		},
	}

	cmd.Flags().StringVar(&spiffeID, "spiffe-id", "", "SPIFFE ID")
	cmd.Flags().StringVar(&parentID, "parent-id", "", "Parent SPIFFE ID")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Kubernetes namespace")
	cmd.Flags().StringVar(&sa, "service-account", "", "ServiceAccount")

	cmd.MarkFlagRequired("spiffe-id")
	cmd.MarkFlagRequired("parent-id")
	cmd.MarkFlagRequired("namespace")
	cmd.MarkFlagRequired("service-account")

	return cmd
}

func spireEntryShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show SPIRE entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return spire.EntryShow()
		},
	}
}
