package observability

import (
	"fmt"
	"os"

	"wsldev/internal/observability"

	"github.com/spf13/cobra"
)

func ObservabilityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "observability",
		Short: "Manage observability tools (Prometheus, etc.)",
	}

	cmd.AddCommand(
		prometheusDeployCmd(),
	)

	return cmd
}

func prometheusDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prometheus-deploy",
		Short: "Deploy Prometheus monitoring stack",
		Long:  `Deploy Prometheus and its related Kubernetes manifests to the cluster.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Starting Prometheus deployment...")
			if err := observability.Deploy(); err != nil {
				fmt.Println("Prometheus deployment failed:", err)
				os.Exit(1)
			}
			fmt.Println("Prometheus deployment completed successfully")
		},
	}

	return cmd
}
