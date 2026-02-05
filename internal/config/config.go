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
		Server             string
		Token              string
		InsecureSkipVerify bool
	}

	UI struct {
		SidebarWidth int
	}
}

func Default() Config {
	var c Config
	c.UI.SidebarWidth = 28

	// Common defaults so a port-forward (or local argocd-server) works with minimal config.
	// Argo CD commonly serves HTTPS on 443; port-forward examples often map to https://localhost:8080.
	c.ArgoCD.Server = "https://localhost:8080"
	return c
}

// Load loads configuration from the given path.
//
// Placeholder implementation:
// - If path is empty: returns Default().
// - If path is provided: verifies file exists then returns Default().
//
// It also overlays environment variables so the app can work without a config file.
func Load(path string) (Config, error) {
	c := Default()
	if path != "" {
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return Config{}, err
			}
			return Config{}, err
		}
		// TODO: parse YAML/JSON/TOML from path and populate c.
	}

	// Env overrides (recommended).
	if v := os.Getenv("ARGOCD_SERVER"); v != "" {
		c.ArgoCD.Server = v
	}
	if v := os.Getenv("ARGOCD_AUTH_TOKEN"); v != "" {
		c.ArgoCD.Token = v
	}
	if v := os.Getenv("ARGOCD_INSECURE"); v != "" {
		// Matches argocd CLI: ARGOCD_INSECURE=true
		if v == "1" || v == "true" || v == "TRUE" || v == "yes" || v == "YES" {
			c.ArgoCD.InsecureSkipVerify = true
		}
	}

	return c, nil
}
