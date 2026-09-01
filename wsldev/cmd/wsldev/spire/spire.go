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
	cmd.AddCommand(spireRegisterJVMCmd())

	return cmd
}

func spireDeployCmd() *cobra.Command {
	var attestor string

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy SPIRE to Kubernetes (kubectl apply -k spiffe-spire/overlays/<variant>)",
		Long: `Deploy SPIRE using Kustomize manifests from spiffe-spire/, including:
  - SPIRE server and agent
  - jvm-hashes ConfigMap

Use --attestor to switch between plugin variants:
  - custom-jvm (default): standard k8s/unix attestors + JVM integrity attestor
                           (anti-debug + anti-tamper + jar-hash)
  - default:               standard k8s/unix attestors only (baseline, no JVM plugin)

Switching automatically restarts the spire-agent DaemonSet, since the agent
does not hot-reload its config.

Override the manifests directory with SPIRE_MANIFESTS_PATH if needed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return spire.Deploy(attestor)
		},
	}

	cmd.Flags().StringVar(&attestor, "attestor", spire.AttestorCustomJVM,
		"WorkloadAttestor variant to deploy: default | custom-jvm")

	return cmd
}
