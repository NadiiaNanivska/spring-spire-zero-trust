package spire

import (
	"github.com/spf13/cobra"
	"wsldev/internal/apps"
)

func spireRegisterJVMCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "register-jvm [app...]",
		Short: "Create SPIRE entries for JVM workloads from jvm-hashes-configmap.yaml",
		Long: `Parse spiffe-spire/base/jvm-hashes-configmap.yaml and (re)create the SPIRE
registration entries for the JVM workloads, pinning the expected jar SHA-256 as a
jvm:jar_sha256 selector. With no args, registers all known JVM services.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = apps.JVMServiceNames()
			}
			return apps.RegisterJVMWorkloads(args)
		},
	}
}
