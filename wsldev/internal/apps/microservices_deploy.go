package apps

import (
	"fmt"
	"os"
)

type DeployFunc func() error

var deployRegistry = map[string]DeployFunc{
	"payments": DeployPaymentsPoC,
	// "orders":   DeployOrdersPoC,
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

	if err := RunCmd("docker", "inspect", "registry"); err != nil {
		fmt.Println("Local registry not found, starting registry:2")
		if err := RunCmd(
			"docker",
			"run",
			"-d",
			"-p", "5000:5000",
			"--name", "registry",
			"registry:2",
		); err != nil {
			return err
		}
	}

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

	if err := RunCmd(
		"docker",
		"push",
		"localhost:5000/payments-service:1.0",
	); err != nil {
		return err
	}

	if err := RunCmd(
		"kind",
		"load",
		"docker-image",
		"payments-service:1.0",
	); err != nil {
		return err
	}

	saApp := App{
		Name:      "payments-sa",
		Namespace: "spire",
		Manifests: "/mnt/c/Users/User/Desktop/LNU/Poc/spiffe-spire-quickstart/test-sa.yaml",
	}

	if err := Deploy(saApp); err != nil {
		return err
	}

	svcApp := App{
		Name:      "payment-service-svc",
		Namespace: "spire",
		Manifests: "/mnt/c/Users/User/Desktop/LNU/Poc/payments-service/payment-service-svc.yaml",
	}

	if err := Deploy(svcApp); err != nil {
		return err
	}

	deploymentApp := App{
		Name:      "payment-service-deployment",
		Namespace: "spire",
		Manifests: "/mnt/c/Users/User/Desktop/LNU/Poc/payments-service/payment-service-deployment.yaml",
	}

	if err := Deploy(deploymentApp); err != nil {
		return err
	}

	fmt.Println("payments-service deployed successfully")
	return nil
}
