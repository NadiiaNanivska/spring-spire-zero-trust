package apps

import "fmt"

var appRegistry = map[string]App{
	"payments": {
		Name:      "payments-service",
		Namespace: "spire",
		Manifests: "/mnt/c/Users/User/Desktop/LNU/Poc/payments-service/payment-service-deployment.yaml",
	},
	"payments-sa": {
		Name:      "payments-sa",
		Namespace: "spire",
		Manifests: "/mnt/c/Users/User/Desktop/LNU/Poc/payments-service/payment-service-sa.yaml",
	},
	"payment-service-svc": {
		Name:      "payment-service-svc",
		Namespace: "spire",
		Manifests: "/mnt/c/Users/User/Desktop/LNU/Poc/payments-service/payment-service-svc.yaml",
	},
	"orders": {
		Name:      "orders-service",
		Namespace: "spire",
		Manifests: "/mnt/c/Users/User/Desktop/LNU/Poc/orders-service/order-service-deployment.yaml",
	},
	"orders-sa": {
		Name:      "orders-sa",
		Namespace: "spire",
		Manifests: "/mnt/c/Users/User/Desktop/LNU/Poc/orders-service/order-service-sa.yaml",
	},
}

func GetApp(name string) (App, error) {
	app, ok := appRegistry[name]
	if !ok {
		return App{}, fmt.Errorf("unknown app: %s", name)
	}
	return app, nil
}
