package apps

import (
	"fmt"
	"os"
)

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
		"kubectl",
		"delete",
		"deployment",
		"payments-service",
		"-n", "spire",
	)

	//if err := RunCmd("docker", "inspect", "registry"); err != nil {
	//	fmt.Println("Local registry not found, starting registry:2")
	//	if err := RunCmd(
	//		"docker",
	//		"run",
	//		"-d",
	//		"-p", "5000:5000",
	//		"--name", "registry",
	//		"registry:2",
	//	); err != nil {
	//		return err
	//	}
	//}

	serviceDir := "/mnt/c/Users/User/Desktop/LNU/Poc/payments-service"
	if err := os.Chdir(serviceDir); err != nil {
		return err
	}

	if err := RunCmd("mvn", "clean", "package"); err != nil {
		return err
	}

	if err := RunCmd(
		"docker",
		"build",
		"-t", "payments-service:1.0",
		".",
	); err != nil {
		return err
	}

	if err := RunCmd(
		"docker",
		"tag",
		"payments-service:1.0",
		"localhost:5000/payments-service:1.0",
	); err != nil {
		return err
	}

	//if err := RunCmd(
	//	"docker",
	//	"push",
	//	"localhost:5000/payments-service:1.0",
	//); err != nil {
	//	return err
	//}

	if err := RunCmd(
		"kind",
		"load",
		"docker-image",
		"payments-service:1.0",
	); err != nil {
		return err
	}

	saApp, _ := GetApp("payments-sa")

	if err := Deploy(saApp); err != nil {
		return err
	}

	svcApp, _ := GetApp("payment-service-svc")

	if err := Deploy(svcApp); err != nil {
		return err
	}

	deploymentApp, _ := GetApp("payments")

	if err := Deploy(deploymentApp); err != nil {
		return err
	}

	fmt.Println("payments-service deployed successfully")
	return nil
}

func DeployOrdersPoC() error {
	fmt.Println("Deploying orders-service (PoC)")

	_ = RunCmd(
		"kubectl",
		"delete",
		"deployment",
		"orders-service",
		"-n", "spire",
	)

	//if err := RunCmd("docker", "inspect", "registry"); err != nil {
	//	fmt.Println("Local registry not found, starting registry:2")
	//	if err := RunCmd(
	//		"docker",
	//		"run",
	//		"-d",
	//		"-p", "5000:5000",
	//		"--name", "registry",
	//		"registry:2",
	//	); err != nil {
	//		return err
	//	}
	//}

	serviceDir := "/mnt/c/Users/User/Desktop/LNU/Poc/orders-service"
	if err := os.Chdir(serviceDir); err != nil {
		return err
	}

	if err := RunCmd("mvn", "clean", "package"); err != nil {
		return err
	}

	if err := RunCmd(
		"docker",
		"build",
		"-t", "orders-service:1.0",
		".",
	); err != nil {
		return err
	}

	if err := RunCmd(
		"docker",
		"tag",
		"orders-service:1.0",
		"localhost:5000/orders-service:1.0",
	); err != nil {
		return err
	}

	//if err := RunCmd(
	//	"docker",
	//	"push",
	//	"localhost:5000/orders-service:1.0",
	//); err != nil {
	//	return err
	//}

	if err := RunCmd(
		"kind",
		"load",
		"docker-image",
		"orders-service:1.0",
	); err != nil {
		return err
	}

	saApp, _ := GetApp("orders-sa")

	if err := Deploy(saApp); err != nil {
		return err
	}

	deploymentApp, _ := GetApp("orders")

	if err := Deploy(deploymentApp); err != nil {
		return err
	}

	fmt.Println("orders-service deployed successfully")
	return nil
}
