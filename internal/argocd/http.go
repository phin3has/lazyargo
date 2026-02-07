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
	"sort"
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
			OperationState *struct {
				Phase   string `json:"phase"`
				Message string `json:"message"`
			} `json:"operationState"`
			History []struct {
				Revision   string `json:"revision"`
				DeployedAt string `json:"deployedAt"`
				DeployStartedAt string `json:"deployStartedAt"`
				Source     any    `json:"source"`
			} `json:"history"`
			Resources []struct {
				Group     string `json:"group"`
				Kind      string `json:"kind"`
				Version   string `json:"version"`
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
			Version:   r.Version,
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
			Version    string `json:"version"`
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
				Version:   n.Version,
				Name:      n.Name,
				Namespace: n.Namespace,
				Status:    status,
				Health:    n.Health.Status,
				Hook:      n.Hook,
			})
		}
	}

	var op *OperationState
	if resp.Status.OperationState != nil {
		op = &OperationState{Phase: resp.Status.OperationState.Phase, Message: resp.Status.OperationState.Message}
	}

	history := make([]SyncHistoryEntry, 0, len(resp.Status.History))
	for _, h := range resp.Status.History {
		deployedAt := h.DeployedAt
		if deployedAt == "" {
			deployedAt = h.DeployStartedAt
		}
		src := ""
		if h.Source != nil {
			if b, err := json.Marshal(h.Source); err == nil {
				src = string(b)
			}
		}
		history = append(history, SyncHistoryEntry{Revision: h.Revision, DeployedAt: deployedAt, Status: "", Message: "", Source: src})
	}

	return Application{
		Name:           resp.Metadata.Name,
		Namespace:      resp.Spec.Destination.Namespace,
		Project:        resp.Spec.Project,
		Health:         resp.Status.Health.Status,
		Sync:           resp.Status.Sync.Status,
		RepoURL:        resp.Spec.Source.RepoURL,
		Revision:       resp.Spec.Source.TargetRevision,
		Path:           resp.Spec.Source.Path,
		Cluster:        resp.Spec.Destination.Server,
		Resources:      resources,
		OperationState: op,
		History:        history,
	}, nil
}

func (c *HTTPClient) ListRevisions(ctx context.Context, name string) ([]Revision, error) {
	if err := c.ensureLogin(ctx); err != nil {
		return nil, err
	}

	// First fetch application history IDs and their git revisions.
	var app struct {
		Status struct {
			History []struct {
				ID       int64  `json:"id"`
				Revision string `json:"revision"`
			} `json:"history"`
		} `json:"status"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/applications/"+url.PathEscape(name), nil, &app); err != nil {
		return nil, err
	}

	revs := make([]Revision, 0, len(app.Status.History))
	for _, h := range app.Status.History {
		r := Revision{ID: h.ID, Revision: h.Revision}
		if h.Revision != "" {
			var meta struct {
				Author  string `json:"author"`
				Date    string `json:"date"`
				Message string `json:"message"`
			}
			_ = c.doJSON(ctx, http.MethodGet, "/api/v1/applications/"+url.PathEscape(name)+"/revisions/"+url.PathEscape(h.Revision)+"/metadata", nil, &meta)
			r.Author = meta.Author
			r.Date = meta.Date
			r.Message = meta.Message
		}
		revs = append(revs, r)
	}

	// Newest first.
	sort.SliceStable(revs, func(i, j int) bool { return revs[i].ID > revs[j].ID })
	if len(revs) > 20 {
		revs = revs[:20]
	}
	return revs, nil
}

func (c *HTTPClient) RollbackApplication(ctx context.Context, name string, revisionID int64) error {
	if err := c.ensureLogin(ctx); err != nil {
		return err
	}
	payload := struct {
		ID int64 `json:"id"`
	}{ID: revisionID}
	return c.doJSON(ctx, http.MethodPost, "/api/v1/applications/"+url.PathEscape(name)+"/rollback", payload, nil)
}

func (c *HTTPClient) TerminateOperation(ctx context.Context, name string) error {
	if err := c.ensureLogin(ctx); err != nil {
		return err
	}
	return c.doJSON(ctx, http.MethodDelete, "/api/v1/applications/"+url.PathEscape(name)+"/operation", nil, nil)
}

func (c *HTTPClient) CreateApplication(ctx context.Context, app Application) error {
	if err := c.ensureLogin(ctx); err != nil {
		return err
	}

	spec := map[string]any{
		"metadata": map[string]any{
			"name": app.Name,
		},
		"spec": map[string]any{
			"project": app.Project,
			"source": map[string]any{
				"repoURL":        app.RepoURL,
				"path":           app.Path,
				"targetRevision": app.Revision,
			},
			"destination": map[string]any{
				"server":    app.Cluster,
				"namespace": app.Namespace,
			},
		},
	}

	if strings.EqualFold(app.SyncPolicy, "auto") {
		specSpec := spec["spec"].(map[string]any)
		specSpec["syncPolicy"] = map[string]any{
			"automated": map[string]any{},
		}
	}

	return c.doJSON(ctx, http.MethodPost, "/api/v1/applications", spec, nil)
}

func (c *HTTPClient) ListProjects(ctx context.Context) ([]string, error) {
	if err := c.ensureLogin(ctx); err != nil {
		return nil, err
	}
	var resp struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/projects", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp.Items))
	for _, it := range resp.Items {
		if it.Metadata.Name != "" {
			out = append(out, it.Metadata.Name)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (c *HTTPClient) ListClusters(ctx context.Context) ([]string, error) {
	if err := c.ensureLogin(ctx); err != nil {
		return nil, err
	}
	var resp []struct {
		Server string `json:"server"`
		Name   string `json:"name"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/clusters", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp))
	for _, c := range resp {
		if c.Server != "" {
			out = append(out, c.Server)
		} else if c.Name != "" {
			out = append(out, c.Name)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (c *HTTPClient) ListRepositories(ctx context.Context) ([]string, error) {
	if err := c.ensureLogin(ctx); err != nil {
		return nil, err
	}
	var resp []struct {
		Repo string `json:"repo"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/repositories", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp))
	for _, r := range resp {
		if r.Repo != "" {
			out = append(out, r.Repo)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (c *HTTPClient) UpdateApplication(ctx context.Context, app Application) error {
	if err := c.ensureLogin(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(app.Name) == "" {
		return fmt.Errorf("missing application name")
	}

	payload := map[string]any{
		"metadata": map[string]any{
			"name": app.Name,
		},
		"spec": map[string]any{
			"project": app.Project,
			"source": map[string]any{
				"repoURL":        app.RepoURL,
				"path":           app.Path,
				"targetRevision": app.Revision,
			},
			"destination": map[string]any{
				"server":    app.Cluster,
				"namespace": app.Namespace,
			},
		},
	}
	if strings.EqualFold(app.SyncPolicy, "auto") {
		payload["spec"].(map[string]any)["syncPolicy"] = map[string]any{"automated": map[string]any{}}
	}

	return c.doJSON(ctx, http.MethodPut, "/api/v1/applications/"+url.PathEscape(app.Name), payload, nil)
}

func (c *HTTPClient) DeleteApplication(ctx context.Context, name string, cascade bool) error {
	if err := c.ensureLogin(ctx); err != nil {
		return err
	}
	path := "/api/v1/applications/" + url.PathEscape(name)
	if cascade {
		path += "?cascade=true"
	}
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil)
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

func (c *HTTPClient) GetResource(ctx context.Context, appName string, resource ResourceRef) (string, error) {
	if err := c.ensureLogin(ctx); err != nil {
		return "", err
	}

	q := url.Values{}
	q.Set("namespace", resource.Namespace)
	q.Set("resourceName", resource.Name)
	q.Set("version", resource.Version)
	q.Set("kind", resource.Kind)
	q.Set("group", resource.Group)

	path := "/api/v1/applications/" + url.PathEscape(appName) + "/resource?" + q.Encode()
	var resp struct {
		Manifest string `json:"manifest"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return "", err
	}
	return resp.Manifest, nil
}

func (c *HTTPClient) GetManifests(ctx context.Context, appName string) ([]string, error) {
	if err := c.ensureLogin(ctx); err != nil {
		return nil, err
	}
	var resp struct {
		Manifests []string `json:"manifests"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/applications/"+url.PathEscape(appName)+"/manifests", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Manifests, nil
}

func (c *HTTPClient) ListEvents(ctx context.Context, appName string) ([]Event, error) {
	if err := c.ensureLogin(ctx); err != nil {
		return nil, err
	}
	var resp struct {
		Items []struct {
			Type    string `json:"type"`
			Reason  string `json:"reason"`
			Message string `json:"message"`
			// Kubernetes Event fields vary by version.
			LastTimestamp     string `json:"lastTimestamp"`
			EventTime         string `json:"eventTime"`
			FirstTimestamp    string `json:"firstTimestamp"`
			CreationTimestamp string `json:"creationTimestamp"`
			InvolvedObject    struct {
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"involvedObject"`
		} `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/applications/"+url.PathEscape(appName)+"/events", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(resp.Items))
	for _, it := range resp.Items {
		ts := strings.TrimSpace(it.LastTimestamp)
		if ts == "" {
			ts = strings.TrimSpace(it.EventTime)
		}
		if ts == "" {
			ts = strings.TrimSpace(it.CreationTimestamp)
		}
		if ts == "" {
			ts = strings.TrimSpace(it.FirstTimestamp)
		}
		obj := strings.TrimSpace(it.InvolvedObject.Kind)
		if it.InvolvedObject.Name != "" {
			obj += "/" + it.InvolvedObject.Name
		}
		if it.InvolvedObject.Namespace != "" {
			obj += " (" + it.InvolvedObject.Namespace + ")"
		}
		out = append(out, Event{Type: it.Type, Reason: it.Reason, Message: it.Message, Timestamp: ts, InvolvedObject: obj})
	}
	return out, nil
}

func (c *HTTPClient) PodLogs(ctx context.Context, appName, podName, container string, follow bool) (io.ReadCloser, error) {
	if err := c.ensureLogin(ctx); err != nil {
		return nil, err
	}

	u, err := url.Parse(c.Server)
	if err != nil {
		return nil, fmt.Errorf("invalid server url: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/v1/applications/" + url.PathEscape(appName) + "/pods/" + url.PathEscape(podName) + "/logs"
	q := u.Query()
	if container != "" {
		q.Set("container", container)
	}
	if follow {
		q.Set("follow", "true")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if tok := c.token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	res, err := c.client().Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		return nil, fmt.Errorf("argocd api GET logs failed: %s: %s", res.Status, strings.TrimSpace(string(b)))
	}
	// Caller must close.
	return res.Body, nil
}

func (c *HTTPClient) ServerSideDiff(ctx context.Context, appName string) ([]DiffResult, error) {
	if err := c.ensureLogin(ctx); err != nil {
		return nil, err
	}

	type diffItem struct {
		Diff     string `json:"diff"`
		Modified bool   `json:"modified"`
		Resource struct {
			Group     string `json:"group"`
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			Version   string `json:"version"`
		} `json:"resource"`
	}

	// The API shape varies across Argo CD versions.
	var resp struct {
		Items []diffItem `json:"items"`
		Diffs []diffItem `json:"diffs"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/applications/"+url.PathEscape(appName)+"/server-side-diff", nil, &resp); err != nil {
		return nil, err
	}

	items := resp.Items
	if len(items) == 0 {
		items = resp.Diffs
	}

	out := make([]DiffResult, 0, len(items))
	for _, it := range items {
		out = append(out, DiffResult{
			Ref: ResourceRef{Group: it.Resource.Group, Kind: it.Resource.Kind, Name: it.Resource.Name, Namespace: it.Resource.Namespace, Version: it.Resource.Version},
			Diff:     it.Diff,
			Modified: it.Modified,
		})
	}
	return out, nil
}

func (c *HTTPClient) RevisionMetadata(ctx context.Context, appName, revision string) (RevisionMeta, error) {
	_ = ctx
	_ = appName
	_ = revision
	return RevisionMeta{}, fmt.Errorf("revision metadata not implemented")
}

func (c *HTTPClient) ChartDetails(ctx context.Context, appName, revision string) (ChartMeta, error) {
	_ = ctx
	_ = appName
	_ = revision
	return ChartMeta{}, fmt.Errorf("chart details not implemented")
}

func (c *HTTPClient) GetSyncWindows(ctx context.Context, appName string) ([]SyncWindow, error) {
	_ = ctx
	_ = appName
	return nil, fmt.Errorf("sync windows not implemented")
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
