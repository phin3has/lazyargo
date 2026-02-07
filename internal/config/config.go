package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the app configuration.
//
// Keep this small initially; grow it as the UI and Argo CD integration evolve.
type Config struct {
	ArgoCD struct {
		Server             string `yaml:"server"`
		Token              string `yaml:"token"`
		InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
	} `yaml:"argocd"`

	UI struct {
		SidebarWidth int `yaml:"sidebarWidth"`
	} `yaml:"ui"`

	LogLevel string `yaml:"logLevel"`
}

func Default() Config {
	var c Config
	c.UI.SidebarWidth = 28
	c.LogLevel = "info"

	// Common defaults so a port-forward (or local argocd-server) works with minimal config.
	// Argo CD commonly serves HTTPS on 443; port-forward examples often map to https://localhost:8080.
	c.ArgoCD.Server = "https://localhost:8080"
	return c
}

func defaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(dir, "lazyargo", "config.yaml"), nil
}

func parseBoolish(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s == "1" || s == "true" || s == "yes" || s == "y" || s == "on"
}

// Load loads configuration from the given path.
//
// Precedence (highest → lowest):
//  1. Environment variables (ARGOCD_*)
//  2. YAML file (if provided, or if default path exists)
//  3. Defaults
func Load(path string) (Config, error) {
	c := Default()

	// If no explicit path was provided, attempt the default config path (optional).
	if path == "" {
		p, err := defaultPath()
		if err == nil {
			if _, statErr := os.Stat(p); statErr == nil {
				path = p
			}
		}
	}

	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return Config{}, err
			}
			return Config{}, err
		}

		// Start from defaults and overlay YAML.
		overlay := Default()
		if err := yaml.Unmarshal(b, &overlay); err != nil {
			return Config{}, fmt.Errorf("parse config %q: %w", path, err)
		}

		c = overlay
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
		c.ArgoCD.InsecureSkipVerify = parseBoolish(v)
	}
	if v := os.Getenv("LAZYARGO_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}

	return c, nil
}
