package argocd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	Server     string
	AuthToken  string
	Username   string
	Password   string
	Timeout    time.Duration
	HTTP       *http.Client
	UserAgent  string
	Insecure   bool // placeholder; only relevant when using HTTPS + custom TLS config
	loginToken string
}

func NewHTTPClient(server string) *HTTPClient {
	return &HTTPClient{
		Server:    strings.TrimRight(server, "/"),
		Timeout:   10 * time.Second,
		UserAgent: "lazyargo/0.0.1",
	}
}

func (c *HTTPClient) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: c.Timeout}
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
			Metadata struct{
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct{
				Project     string `json:"project"`
				Destination struct{
					Namespace string `json:"namespace"`
					Server    string `json:"server"`
				} `json:"destination"`
				Source struct{
					RepoURL        string `json:"repoURL"`
					TargetRevision string `json:"targetRevision"`
					Path           string `json:"path"`
				} `json:"source"`
			} `json:"spec"`
			Status struct{
				Health struct{ Status string `json:"status"` } `json:"health"`
				Sync   struct{ Status string `json:"status"` } `json:"sync"`
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
	if err := c.ensureLogin(ctx); err != nil {
		return Application{}, err
	}
	var resp struct {
		Metadata struct{ Name string `json:"name"` } `json:"metadata"`
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
			Health struct{ Status string `json:"status"` } `json:"health"`
			Sync   struct{ Status string `json:"status"` } `json:"sync"`
		} `json:"status"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/applications/"+url.PathEscape(name), nil, &resp); err != nil {
		return Application{}, err
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
	}, nil
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

	res, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	b, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("argocd api %s %s failed: %s: %s", method, path, res.Status, strings.TrimSpace(string(b)))
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
