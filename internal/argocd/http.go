package argocd

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPClient is a minimal Argo CD API client over HTTP.
// It targets the Argo CD REST API used by the web UI/CLI.
//
// API base: <server>/api/v1/
// Login:    POST /api/v1/session {username,password} -> {token}
// Apps:     GET  /api/v1/applications
// App:      GET  /api/v1/applications/{name}
//
// NOTE: This is intentionally small; we'll grow it as the TUI matures.
type HTTPClient struct {
	Server    string
	AuthToken string
	Username  string
	Password  string
	Timeout   time.Duration
	HTTP      *http.Client
	UserAgent string
	Insecure  bool // placeholder; only relevant when using HTTPS + custom TLS config
	Logger    *slog.Logger

	loginToken string
}

func NewHTTPClient(server string) *HTTPClient {
	return &HTTPClient{
		Server:    strings.TrimRight(server, "/"),
		Timeout:   10 * time.Second,
		UserAgent: "lazyargo/0.0.1",
		Logger:    slog.Default(),
	}
}

func (c *HTTPClient) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if c.Insecure {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // explicit user flag
	}

	return &http.Client{Timeout: c.Timeout, Transport: transport}
}

func (c *HTTPClient) token() string {
	if c.AuthToken != "" {
		return c.AuthToken
	}
	return c.loginToken
}

func (c *HTTPClient) ensureLogin(ctx context.Context) error {
	if c.AuthToken != "" {
		return nil
	}
	if c.loginToken != "" {
		return nil
	}
	if c.Username == "" || c.Password == "" {
		return fmt.Errorf("missing Argo CD auth: set ARGOCD_AUTH_TOKEN or provide username/password")
	}

	payload := map[string]string{"username": c.Username, "password": c.Password}
	var out struct {
		Token string `json:"token"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/session", payload, &out); err != nil {
		return err
	}
	if out.Token == "" {
		return fmt.Errorf("argocd login returned empty token")
	}
	c.loginToken = out.Token
	return nil
}

func (c *HTTPClient) ListApplications(ctx context.Context) ([]Application, error) {
	if err := c.ensureLogin(ctx); err != nil {
		return nil, err
	}
	var resp struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				Project     string `json:"project"`
				Destination struct {
					Namespace string `json:"namespace"`
					Server    string `json:"server"`
				} `json:"destination"`
				Source struct {
					RepoURL        string `json:"repoURL"`
					TargetRevision string `json:"targetRevision"`
					Path           string `json:"path"`
				} `json:"source"`
			} `json:"spec"`
			Status struct {
				Health struct {
					Status string `json:"status"`
				} `json:"health"`
				Sync struct {
					Status string `json:"status"`
				} `json:"sync"`
			} `json:"status"`
		} `json:"items"`
	}

	// NOTE: Argo CD returns {metadata:{}, items:[...]}. items can be null.
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/applications", nil, &resp); err != nil {
		return nil, err
	}

	apps := make([]Application, 0, len(resp.Items))
	for _, it := range resp.Items {
		apps = append(apps, Application{
			Name:      it.Metadata.Name,
			Project:   it.Spec.Project,
			Health:    it.Status.Health.Status,
			Sync:      it.Status.Sync.Status,
			RepoURL:   it.Spec.Source.RepoURL,
			Revision:  it.Spec.Source.TargetRevision,
			Path:      it.Spec.Source.Path,
			Namespace: it.Spec.Destination.Namespace,
			Cluster:   it.Spec.Destination.Server,
		})
	}
	return apps, nil
}

func (c *HTTPClient) GetApplication(ctx context.Context, name string) (Application, error) {
	return c.RefreshApplication(ctx, name, false)
}

func (c *HTTPClient) RefreshApplication(ctx context.Context, name string, hard bool) (Application, error) {
	if err := c.ensureLogin(ctx); err != nil {
		return Application{}, err
	}

	path := "/api/v1/applications/" + url.PathEscape(name)
	if hard {
		path += "?refresh=hard"
	}

	var resp struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Project     string `json:"project"`
			Destination struct {
				Namespace string `json:"namespace"`
				Server    string `json:"server"`
			} `json:"destination"`
			Source struct {
				RepoURL        string `json:"repoURL"`
				TargetRevision string `json:"targetRevision"`
				Path           string `json:"path"`
			} `json:"source"`
		} `json:"spec"`
		Status struct {
			Health struct {
				Status string `json:"status"`
			} `json:"health"`
			Sync struct {
				Status string `json:"status"`
			} `json:"sync"`
			Resources []struct {
				Group     string `json:"group"`
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
				Status    string `json:"status"`
				Health    struct {
					Status string `json:"status"`
				} `json:"health"`
				Hook bool `json:"hook"`
			} `json:"resources"`
		} `json:"status"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return Application{}, err
	}
	resources := make([]Resource, 0, len(resp.Status.Resources))
	for _, r := range resp.Status.Resources {
		resources = append(resources, Resource{
			Group:     r.Group,
			Kind:      r.Kind,
			Name:      r.Name,
			Namespace: r.Namespace,
			Status:    r.Status,
			Health:    r.Health.Status,
			Hook:      r.Hook,
		})
	}

	// Prefer the resource tree endpoint for a fuller managed-resource view when available.
	var tree struct {
		Nodes []struct {
			Group      string `json:"group"`
			Kind       string `json:"kind"`
			Name       string `json:"name"`
			Namespace  string `json:"namespace"`
			Status     string `json:"status"`
			SyncStatus string `json:"syncStatus"`
			Health     struct {
				Status string `json:"status"`
			} `json:"health"`
			Hook bool `json:"hook"`
		} `json:"nodes"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/applications/"+url.PathEscape(name)+"/resource-tree", nil, &tree); err == nil && len(tree.Nodes) > 0 {
		resources = resources[:0]
		for _, n := range tree.Nodes {
			status := n.Status
			if status == "" {
				status = n.SyncStatus
			}
			resources = append(resources, Resource{
				Group:     n.Group,
				Kind:      n.Kind,
				Name:      n.Name,
				Namespace: n.Namespace,
				Status:    status,
				Health:    n.Health.Status,
				Hook:      n.Hook,
			})
		}
	}

	return Application{
		Name:      resp.Metadata.Name,
		Namespace: resp.Spec.Destination.Namespace,
		Project:   resp.Spec.Project,
		Health:    resp.Status.Health.Status,
		Sync:      resp.Status.Sync.Status,
		RepoURL:   resp.Spec.Source.RepoURL,
		Revision:  resp.Spec.Source.TargetRevision,
		Path:      resp.Spec.Source.Path,
		Cluster:   resp.Spec.Destination.Server,
		Resources: resources,
	}, nil
}

func (c *HTTPClient) SyncApplication(ctx context.Context, name string, dryRun bool) error {
	if err := c.ensureLogin(ctx); err != nil {
		return err
	}

	payload := struct {
		DryRun bool `json:"dryRun"`
	}{DryRun: dryRun}

	// The Argo CD API returns an Operation object. For now we only care that the request succeeds.
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/applications/"+url.PathEscape(name)+"/sync", payload, nil); err != nil {
		return err
	}
	return nil
}

func (c *HTTPClient) doJSON(ctx context.Context, method, path string, in any, out any) error {
	u, err := url.Parse(c.Server)
	if err != nil {
		return fmt.Errorf("invalid server url: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + path

	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if tok := c.token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	logger := c.Logger
	if logger == nil {
		logger = slog.Default()
	}

	start := time.Now()
	res, err := c.client().Do(req)
	dur := time.Since(start)
	if err != nil {
		// Common local dev case: https://localhost:8080 via port-forward with a cert that isn't trusted.
		hint := ""
		es := err.Error()
		if strings.Contains(es, "x509") || strings.Contains(es, "certificate") {
			hint = " (TLS error: try --insecure or set ARGOCD_INSECURE=true)"
		}

		logger.Error("argocd request failed",
			"method", method,
			"path", path,
			"url", u.String(),
			"duration_ms", dur.Milliseconds(),
			"err", err,
		)
		return fmt.Errorf("argocd request failed: %w%s", err, hint)
	}
	defer res.Body.Close()

	b, _ := io.ReadAll(res.Body)

	logger.Debug("argocd request",
		"method", method,
		"path", path,
		"status", res.StatusCode,
		"duration_ms", dur.Milliseconds(),
	)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(b))
		if len(msg) > 500 {
			msg = msg[:500] + "…"
		}
		logger.Warn("argocd non-2xx response",
			"method", method,
			"path", path,
			"status", res.StatusCode,
			"response", msg,
		)
		return fmt.Errorf("argocd api %s %s failed: %s: %s", method, path, res.Status, msg)
	}
	if out == nil {
		return nil
	}
	if len(b) == 0 {
		return nil
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
