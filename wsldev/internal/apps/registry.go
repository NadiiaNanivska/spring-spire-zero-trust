package apps

import "fmt"

type appDef struct {
	Name           string
	Namespace      string
	ServiceDir     string
	ManifestFile   string
}

var appRegistry = map[string]appDef{
	"payments": {
		Name:         "payments-service",
		Namespace:    "spire",
		ServiceDir:   "payments-service",
		ManifestFile: "payment-service-deployment.yaml",
	},
	"payments-sa": {
		Name:         "payments-sa",
		Namespace:    "spire",
		ServiceDir:   "payments-service",
		ManifestFile: "payment-service-sa.yaml",
	},
	"payment-service-svc": {
		Name:         "payment-service-svc",
		Namespace:    "spire",
		ServiceDir:   "payments-service",
		ManifestFile: "payment-service-svc.yaml",
	},
	"orders": {
		Name:         "orders-service",
		Namespace:    "spire",
		ServiceDir:   "orders-service",
		ManifestFile: "order-service-deployment.yaml",
	},
	"orders-sa": {
		Name:         "orders-sa",
		Namespace:    "spire",
		ServiceDir:   "orders-service",
		ManifestFile: "order-service-sa.yaml",
	},
}

func GetApp(name string) (App, error) {
	def, ok := appRegistry[name]
	if !ok {
		return App{}, fmt.Errorf("unknown app: %s", name)
	}

	dir, err := serviceDir(def.ServiceDir)
	if err != nil {
		return App{}, err
	}

	manifest, err := serviceManifest(dir, def.ManifestFile)
	if err != nil {
		return App{}, err
	}

	return App{
		Name:      def.Name,
		Namespace: def.Namespace,
		Manifests: manifest,
	}, nil
}
