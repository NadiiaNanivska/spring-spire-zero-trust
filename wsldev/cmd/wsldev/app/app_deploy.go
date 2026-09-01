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

			// Recompute the freshly built jar hashes into
			// spiffe-spire/base/jvm-hashes-configmap.yaml. This file is the source
			// of truth for the expected hashes. Best-effort: the apps are already
			// deployed, so a cluster/spire hiccup here should not fail the command.
			if err := apps.SyncJVMHashes(args); err != nil {
				fmt.Println("WARN: jvm hash sync:", err)
			}

			// CI/CD step: turn those expected hashes into SPIRE registration
			// entries (jvm:jar_sha256=<hash> + integrity selectors). The plugin
			// only computes hashes; the server enforces them via these entries.
			if err := apps.RegisterJVMWorkloads(args); err != nil {
				fmt.Println("WARN: jvm workload registration:", err)
			}
		},
	}

	return cmd
}
