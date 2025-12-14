package apps

import (
	"os"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	Apps map[string]App `yaml:"apps"`
}

func LoadApps(path string) (map[string]App, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	for name, app := range cfg.Apps {
		app.Name = name
		cfg.Apps[name] = app
	}

	return cfg.Apps, nil
}
