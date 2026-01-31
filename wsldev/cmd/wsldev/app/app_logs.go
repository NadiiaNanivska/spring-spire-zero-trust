package app

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"wsldev/internal/apps"
)

func appLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs [app-name]",
		Short: "Stream application logs",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			app, err := apps.GetApp(args[0])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			if err := apps.Logs(app); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		},
	}
	return cmd
}
