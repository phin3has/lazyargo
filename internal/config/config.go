package config

import (
	"errors"
	"os"
)

// Config is the app configuration.
//
// Keep this small initially; grow it as the UI and Argo CD integration evolve.
// Eventually load from ~/.config/lazyargo/config.yaml (or similar).
type Config struct {
	ArgoCD struct {
		Server string
		Token  string
		// InsecureSkipVerify bool
	}

	UI struct {
		SidebarWidth int
	}
}

func Default() Config {
	var c Config
	c.UI.SidebarWidth = 28
	return c
}

// Load loads configuration from the given path.
//
// Placeholder implementation:
// - If path is empty: returns Default().
// - If path is provided: verifies file exists then returns Default().
func Load(path string) (Config, error) {
	c := Default()
	if path == "" {
		return c, nil
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, err
		}
		return Config{}, err
	}

	// TODO: parse YAML/JSON/TOML from path and populate c.
	return c, nil
}
