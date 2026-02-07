package main

import (
	"flag"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"lazyargo/internal/argocd"
	"lazyargo/internal/config"
	"lazyargo/internal/ui"
)

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "info", "":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func main() {
	// Set a reasonable default logger early so startup/config errors are structured.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	var (
		configPath string
		useMock    bool
		server     string
		username   string
		password   string
		token      string
		insecure   bool
		logLevel   string
	)

	flag.StringVar(&configPath, "config", "", "path to config file (optional)")
	flag.BoolVar(&useMock, "mock", false, "use mock Argo CD client")
	flag.StringVar(&server, "server", "", "Argo CD server URL (overrides config + ARGOCD_SERVER)")
	flag.StringVar(&username, "username", "", "Argo CD username (or ARGOCD_USERNAME; optional)")
	flag.StringVar(&password, "password", "", "Argo CD password (or ARGOCD_PASSWORD; optional)")
	flag.StringVar(&token, "token", "", "Argo CD auth token (overrides config + ARGOCD_AUTH_TOKEN)")
	flag.BoolVar(&insecure, "insecure", false, "skip TLS verification (or set ARGOCD_INSECURE=true)")
	flag.StringVar(&logLevel, "log-level", "", "log level (debug, info, warn, error)")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	// CLI overrides.
	if server != "" {
		cfg.ArgoCD.Server = server
	}
	if token != "" {
		cfg.ArgoCD.Token = token
	}
	if insecure {
		cfg.ArgoCD.InsecureSkipVerify = true
	}
	if logLevel != "" {
		cfg.LogLevel = logLevel
	}

	// Configure the logger after config+flags are applied.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: parseLogLevel(cfg.LogLevel)})))

	// Username/password are only for future/optional flows.
	usr := firstNonEmpty(username, os.Getenv("ARGOCD_USERNAME"))
	pwd := firstNonEmpty(password, os.Getenv("ARGOCD_PASSWORD"))

	var client argocd.Client
	if useMock || cfg.ArgoCD.Server == "" {
		client = argocd.NewMockClient()
		slog.Info("using mock argocd client")
	} else {
		h := argocd.NewHTTPClient(cfg.ArgoCD.Server)
		h.AuthToken = cfg.ArgoCD.Token
		h.Username = usr
		h.Password = pwd
		h.Insecure = cfg.ArgoCD.InsecureSkipVerify
		client = h
	}

	m := ui.NewModel(cfg, client)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		slog.Error("tui exited with error", "err", err)
		os.Exit(1)
	}
}
