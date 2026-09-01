package apps

import (
	"fmt"
	"os"
)

const kindCluster = "kind"

type DeployFunc func() error

var deployRegistry = map[string]DeployFunc{
	"payments": DeployPaymentsPoC,
	"orders":   DeployOrdersPoC,
}

func DeployByName(name string) error {
	deployFn, ok := deployRegistry[name]
	if !ok {
		return fmt.Errorf("no deploy pipeline defined for app: %s", name)
	}
	return deployFn()
}

func DeployPaymentsPoC() error {
	fmt.Println("Deploying payments-service (PoC)")

	_ = RunCmd(
		"kubectl", "delete", "deployment", "payments-service",
		"-n", "spire", "--ignore-not-found=true",
	)

	serviceDir, err := serviceDir("payments-service")
	if err != nil {
		return err
	}
	if err := os.Chdir(serviceDir); err != nil {
		return err
	}

	if err := RunCmd("mvn", "clean", "package", "-DskipTests"); err != nil {
		return err
	}

	if err := RunCmd("docker", "build", "-t", "payments-service:1.0", "."); err != nil {
		return err
	}

	if err := RunCmd(
		"kind", "load", "docker-image", "payments-service:1.0",
		"--name", kindCluster,
	); err != nil {
		return err
	}

	saApp, err := GetApp("payments-sa")
	if err != nil {
		return err
	}
	if err := Deploy(saApp); err != nil {
		return err
	}

	svcApp, err := GetApp("payment-service-svc")
	if err != nil {
		return err
	}
	if err := Deploy(svcApp); err != nil {
		return err
	}

	deploymentApp, err := GetApp("payments")
	if err != nil {
		return err
	}
	if err := Deploy(deploymentApp); err != nil {
		return err
	}

	fmt.Println("payments-service deployed successfully")
	return nil
}

func DeployOrdersPoC() error {
	fmt.Println("Deploying orders-service (PoC)")

	_ = RunCmd(
		"kubectl", "delete", "deployment", "orders-service",
		"-n", "spire", "--ignore-not-found=true",
	)

	serviceDir, err := serviceDir("orders-service")
	if err != nil {
		return err
	}
	if err := os.Chdir(serviceDir); err != nil {
		return err
	}

	if err := RunCmd("mvn", "clean", "package", "-DskipTests"); err != nil {
		return err
	}

	if err := RunCmd("docker", "build", "-t", "orders-service:1.0", "."); err != nil {
		return err
	}

	if err := RunCmd(
		"kind", "load", "docker-image", "orders-service:1.0",
		"--name", kindCluster,
	); err != nil {
		return err
	}

	saApp, err := GetApp("orders-sa")
	if err != nil {
		return err
	}
	if err := Deploy(saApp); err != nil {
		return err
	}

	deploymentApp, err := GetApp("orders")
	if err != nil {
		return err
	}
	if err := Deploy(deploymentApp); err != nil {
		return err
	}

	fmt.Println("orders-service deployed successfully")
	return nil
}
