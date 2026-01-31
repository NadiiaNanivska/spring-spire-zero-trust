package app

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"wsldev/internal/apps"
)

func appPortForwardCmd() *cobra.Command {
	var ports string

	cmd := &cobra.Command{
		Use:   "port-forward [app-name]",
		Short: "Forward ports to local machine",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if ports == "" {
				fmt.Println("--ports is required")
				os.Exit(1)
			}

			app, err := apps.GetApp(args[0])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			if err := apps.PortForward(app, ports); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&ports, "ports", "", "Local:Remote ports")
	return cmd
}
