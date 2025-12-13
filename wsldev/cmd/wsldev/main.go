package main

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "wsldev/internal/kubernetes"
    "wsldev/internal/docker"
)

func main() {
    rootCmd := &cobra.Command{
        Use:   "wsldev",
        Short: "WSL DevOps Helper",
    }

    // ----------------------
    // Docker daemon
    // ----------------------
    daemonCmd := &cobra.Command{
        Use:   "daemon",
        Short: "Управління Docker daemon у WSL",
    }

    startCmd := &cobra.Command{
        Use:   "start",
        Short: "Запустити dockerd",
        Run: func(cmd *cobra.Command, args []string) {
            err := docker.StartDockerd()
            if err != nil {
                fmt.Println("Error:", err)
                os.Exit(1)
            }
            fmt.Println("Docker daemon запущено")
        },
    }

    statusCmd := &cobra.Command{
        Use:   "status",
        Short: "Перевірити чи працює dockerd",
        Run: func(cmd *cobra.Command, args []string) {
            running, err := docker.IsDockerdRunning()
            if err != nil {
                fmt.Println("Error:", err)
                os.Exit(1)
            }
            if running {
                fmt.Println("Docker daemon працює ✅")
            } else {
                fmt.Println("Docker daemon не запущено ❌")
            }
        },
    }

    daemonCmd.AddCommand(startCmd, statusCmd)
    rootCmd.AddCommand(daemonCmd)

    // ----------------------
    // Kubernetes Kind
    // ----------------------
	clusterCmd := &cobra.Command{
		Use:   "cluster",
		Short: "Управління Kind кластером",
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Створити кластер",
		Run: func(cmd *cobra.Command, args []string) {
			name, _ := cmd.Flags().GetString("name")
			cm := kubernetes.NewClusterManager(name)
			err := cm.Create()
			if err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			fmt.Println("Cluster created ✅")
		},
	}

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Видалити кластер",
		Run: func(cmd *cobra.Command, args []string) {
			name, _ := cmd.Flags().GetString("name")
			cm := kubernetes.NewClusterManager(name)
			err := cm.Delete()
			if err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			fmt.Println("Cluster deleted ✅")
		},
	}

	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: "Повний reset кластера",
		Run: func(cmd *cobra.Command, args []string) {
			name, _ := cmd.Flags().GetString("name")
			cm := kubernetes.NewClusterManager(name)
			err := cm.Reset()
			if err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			fmt.Println("Cluster reset ✅")
		},
	}

	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Інформація про кластер",
		Run: func(cmd *cobra.Command, args []string) {
			name, _ := cmd.Flags().GetString("name")
			cm := kubernetes.NewClusterManager(name)
			err := cm.Info()
			if err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
		},
	}

	createCmd.Flags().String("name", "kind", "Назва кластера")
	deleteCmd.Flags().String("name", "kind", "Назва кластера")
	resetCmd.Flags().String("name", "kind", "Назва кластера")
	infoCmd.Flags().String("name", "kind", "Назва кластера")

	clusterCmd.AddCommand(createCmd, deleteCmd, resetCmd, infoCmd)
	rootCmd.AddCommand(clusterCmd)

	// ----------------------
    // Environment setup: docker and kubectl cluster
    // ----------------------
	upCmd := &cobra.Command{
	Use:   "up",
	Short: "Підняти Docker та Kubernetes кластер",
	Run: func(cmd *cobra.Command, args []string) {
			name, _ := cmd.Flags().GetString("name")
			cm := kubernetes.NewClusterManager(name)

			// 1️⃣ Docker
			running, err := docker.IsDockerdRunning()
			if err != nil {
				fmt.Println("Error checking Docker:", err)
				os.Exit(1)
			}
			if !running {
				fmt.Println("Docker не запущено, запускаємо...")
				err := docker.StartDockerd()
				if err != nil {
					fmt.Println("Error starting Docker:", err)
					os.Exit(1)
				}
				fmt.Println("Docker запущено ✅")
			} else {
				fmt.Println("Docker вже запущено ✅")
			}

			// 2️⃣ Kind кластер
			exists, err := cm.Exists()
			if err != nil {
				fmt.Println("Error checking cluster:", err)
				os.Exit(1)
			}
			if !exists {
				fmt.Println("Kind кластер не знайдено, створюємо...")
				if err := cm.Create(); err != nil {
					fmt.Println("Error creating cluster:", err)
					os.Exit(1)
				}
				fmt.Println("Kind кластер створено ✅")
			} else {
				fmt.Println("Kind кластер вже існує ✅")
			}

			// 3️⃣ Info
			fmt.Println("Інформація про кластер:")
			if err := cm.Info(); err != nil {
				fmt.Println("Error getting cluster info:", err)
				os.Exit(1)
			}
		},
	}

	upCmd.Flags().String("name", "kind", "Назва кластера")
	rootCmd.AddCommand(upCmd)

    // ----------------------
    // Execute
    // ----------------------
    if err := rootCmd.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}
