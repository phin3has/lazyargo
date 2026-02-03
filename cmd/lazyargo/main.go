package main

import (
	"flag"
	"fmt"
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

func main() {
	var (
		configPath string
		useMock    bool
		server    string
		username  string
		password  string
		token     string
	)

	flag.StringVar(&configPath, "config", "", "path to config file (optional)")
	flag.BoolVar(&useMock, "mock", true, "use mock Argo CD client (default: true)")
	flag.StringVar(&server, "server", "", "Argo CD server URL (or ARGOCD_SERVER)")
	flag.StringVar(&username, "username", "", "Argo CD username (or ARGOCD_USERNAME)")
	flag.StringVar(&password, "password", "", "Argo CD password (or ARGOCD_PASSWORD)")
	flag.StringVar(&token, "token", "", "Argo CD auth token (or ARGOCD_AUTH_TOKEN)")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	var client argocd.Client
	if useMock {
		client = argocd.NewMockClient()
	} else {
		srv := firstNonEmpty(server, os.Getenv("ARGOCD_SERVER"))
		usr := firstNonEmpty(username, os.Getenv("ARGOCD_USERNAME"))
		pwd := firstNonEmpty(password, os.Getenv("ARGOCD_PASSWORD"))
		tok := firstNonEmpty(token, os.Getenv("ARGOCD_AUTH_TOKEN"))
		if srv == "" {
			fmt.Fprintln(os.Stderr, "missing Argo CD server: set --server or ARGOCD_SERVER")
			os.Exit(2)
		}
		h := argocd.NewHTTPClient(srv)
		h.AuthToken = tok
		h.Username = usr
		h.Password = pwd
		client = h
	}

	m := ui.NewModel(cfg, client)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
