package apps

import "fmt"

var appRegistry = map[string]App{
	"payments": {
		Name:      "payments-service",
		Namespace: "spire",
		Manifests: "/mnt/c/Users/User/Desktop/LNU/Poc/payment-service-deployment.yaml",
	},
	// "orders": { ... }
}

func GetApp(name string) (App, error) {
	app, ok := appRegistry[name]
	if !ok {
		return App{}, fmt.Errorf("unknown app: %s", name)
	}
	return app, nil
}
