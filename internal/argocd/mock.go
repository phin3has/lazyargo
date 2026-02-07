package argocd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

type MockClient struct {
	apps []Application
}

func NewMockClient() *MockClient {
	return &MockClient{apps: []Application{
		{
			Name:      "payments-api",
			Namespace: "payments",
			Project:   "default",
			Health:    "Healthy",
			Sync:      "Synced",
			RepoURL:   "https://github.com/example/platform",
			Path:      "apps/payments",
			Revision:  "main",
			Cluster:   "https://kubernetes.default.svc",
			Resources: []Resource{
				{Group: "apps", Kind: "Deployment", Version: "v1", Name: "payments-api", Namespace: "payments", Status: "Synced", Health: "Healthy"},
				{Group: "", Kind: "Service", Version: "v1", Name: "payments-api", Namespace: "payments", Status: "Synced", Health: "Healthy"},
				{Group: "", Kind: "ConfigMap", Version: "v1", Name: "payments-config", Namespace: "payments", Status: "Synced", Health: "Healthy"},
				{Group: "autoscaling", Kind: "HorizontalPodAutoscaler", Version: "v2", Name: "payments-api", Namespace: "payments", Status: "Synced", Health: "Healthy"},
			},
		},
		{
			Name:           "orders-worker",
			Namespace:      "orders",
			Project:        "default",
			Health:         "Progressing",
			Sync:           "Synced",
			OperationState: &OperationState{Phase: "Running", Message: "syncing"},
			RepoURL:        "https://github.com/example/platform",
			Path:           "apps/orders",
			Revision:       "main",
			Cluster:        "https://kubernetes.default.svc",
			Resources: []Resource{
				{Group: "apps", Kind: "Deployment", Version: "v1", Name: "orders-worker", Namespace: "orders", Status: "Synced", Health: "Progressing"},
				{Group: "batch", Kind: "CronJob", Version: "v1", Name: "orders-reconciler", Namespace: "orders", Status: "Synced", Health: "Healthy"},
			},
		},
		{
			Name:      "web-frontend",
			Namespace: "web",
			Project:   "default",
			Health:    "Healthy",
			Sync:      "OutOfSync",
			RepoURL:   "https://github.com/example/platform",
			Path:      "apps/web",
			Revision:  "main",
			Cluster:   "https://kubernetes.default.svc",
			Resources: []Resource{
				{Group: "apps", Kind: "Deployment", Version: "v1", Name: "web-frontend", Namespace: "web", Status: "OutOfSync", Health: "Healthy"},
				{Group: "", Kind: "Service", Version: "v1", Name: "web-frontend", Namespace: "web", Status: "Synced", Health: "Healthy"},
				{Group: "networking.k8s.io", Kind: "Ingress", Version: "v1", Name: "web", Namespace: "web", Status: "OutOfSync", Health: "Healthy"},
				{Group: "", Kind: "Secret", Version: "v1", Name: "web-tls", Namespace: "web", Status: "OutOfSync", Health: "—"},
			},
		},
		{
			Name:      "observability",
			Namespace: "ops",
			Project:   "platform",
			Health:    "Degraded",
			Sync:      "Synced",
			RepoURL:   "https://github.com/example/ops",
			Path:      "apps/observability",
			Revision:  "main",
			Cluster:   "https://kubernetes.default.svc",
			Resources: []Resource{
				{Group: "apps", Kind: "StatefulSet", Version: "v1", Name: "loki", Namespace: "ops", Status: "Synced", Health: "Degraded"},
				{Group: "apps", Kind: "Deployment", Version: "v1", Name: "grafana", Namespace: "ops", Status: "Synced", Health: "Healthy"},
				{Group: "", Kind: "Service", Version: "v1", Name: "grafana", Namespace: "ops", Status: "Synced", Health: "Healthy"},
				{Group: "", Kind: "Job", Version: "v1", Name: "migrate-dashboards", Namespace: "ops", Status: "Synced", Health: "Healthy", Hook: true},
			},
		},
		{
			Name:      "cluster-addons",
			Namespace: "kube-system",
			Project:   "platform",
			Health:    "Missing",
			Sync:      "Unknown",
			RepoURL:   "https://github.com/example/ops",
			Path:      "clusters/dev/addons",
			Revision:  "v1.2.3",
			Cluster:   "https://kubernetes.default.svc",
			Resources: []Resource{
				{Group: "apps", Kind: "DaemonSet", Version: "v1", Name: "node-exporter", Namespace: "kube-system", Status: "Unknown", Health: "Missing"},
				{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole", Version: "v1", Name: "addons-read", Namespace: "", Status: "Unknown", Health: "—"},
			},
		},
	}}
}

func (m *MockClient) ListApplications(ctx context.Context) ([]Application, error) {
	_ = ctx
	out := make([]Application, len(m.apps))
	copy(out, m.apps)
	return out, nil
}

func (m *MockClient) GetApplication(ctx context.Context, name string) (Application, error) {
	return m.RefreshApplication(ctx, name, false)
}

func (m *MockClient) RefreshApplication(ctx context.Context, name string, hard bool) (Application, error) {
	_ = ctx
	_ = hard
	for _, a := range m.apps {
		if a.Name == name {
			return a, nil
		}
	}
	return Application{}, fmt.Errorf("application not found: %s", name)
}

func (m *MockClient) ListRevisions(ctx context.Context, name string) ([]Revision, error) {
	_ = ctx
	// Use a stable sample history for the demo.
	for _, a := range m.apps {
		if a.Name == name {
			return []Revision{
				{ID: 3, Revision: "f00dbabe", Author: "alice", Date: "2026-02-01T12:34:56Z", Message: "bump image tag"},
				{ID: 2, Revision: "deadbeef", Author: "bob", Date: "2026-01-28T09:15:00Z", Message: "fix values"},
				{ID: 1, Revision: "c0ffee", Author: "ci", Date: "2026-01-20T18:00:00Z", Message: "initial deploy"},
			}, nil
		}
	}
	return nil, fmt.Errorf("application not found: %s", name)
}

func (m *MockClient) RollbackApplication(ctx context.Context, name string, revisionID int64) error {
	_ = ctx
	for i := range m.apps {
		if m.apps[i].Name == name {
			m.apps[i].Sync = "OutOfSync"
			_ = revisionID
			return nil
		}
	}
	return fmt.Errorf("application not found: %s", name)
}

func (m *MockClient) TerminateOperation(ctx context.Context, name string) error {
	_ = ctx
	for i := range m.apps {
		if m.apps[i].Name == name {
			m.apps[i].OperationState = nil
			return nil
		}
	}
	return fmt.Errorf("application not found: %s", name)
}

func (m *MockClient) DeleteApplication(ctx context.Context, name string, cascade bool) error {
	_ = ctx
	_ = cascade
	for i := range m.apps {
		if m.apps[i].Name == name {
			m.apps = append(m.apps[:i], m.apps[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("application not found: %s", name)
}

func (m *MockClient) CreateApplication(ctx context.Context, app Application) error {
	_ = ctx
	if app.Name == "" {
		return fmt.Errorf("missing application name")
	}
	for _, a := range m.apps {
		if a.Name == app.Name {
			return fmt.Errorf("application already exists: %s", app.Name)
		}
	}
	if app.Project == "" {
		app.Project = "default"
	}
	m.apps = append(m.apps, app)
	return nil
}

func (m *MockClient) ListProjects(ctx context.Context) ([]string, error) {
	_ = ctx
	return []string{"default", "platform"}, nil
}

func (m *MockClient) ListClusters(ctx context.Context) ([]string, error) {
	_ = ctx
	return []string{"https://kubernetes.default.svc"}, nil
}

func (m *MockClient) ListRepositories(ctx context.Context) ([]string, error) {
	_ = ctx
	return []string{"https://github.com/example/platform", "https://github.com/example/ops"}, nil
}

func (m *MockClient) UpdateApplication(ctx context.Context, app Application) error {
	_ = ctx
	for i := range m.apps {
		if m.apps[i].Name == app.Name {
			m.apps[i].Project = app.Project
			m.apps[i].RepoURL = app.RepoURL
			m.apps[i].Path = app.Path
			m.apps[i].Revision = app.Revision
			m.apps[i].Cluster = app.Cluster
			m.apps[i].Namespace = app.Namespace
			m.apps[i].SyncPolicy = app.SyncPolicy
			return nil
		}
	}
	return fmt.Errorf("application not found: %s", app.Name)
}

func (m *MockClient) SyncApplication(ctx context.Context, name string, dryRun bool) error {
	_ = ctx
	for i := range m.apps {
		if m.apps[i].Name != name {
			continue
		}
		if dryRun {
			return nil
		}
		m.apps[i].Sync = "Synced"
		for r := range m.apps[i].Resources {
			if m.apps[i].Resources[r].Status != "Synced" {
				m.apps[i].Resources[r].Status = "Synced"
			}
		}
		return nil
	}
	return fmt.Errorf("application not found: %s", name)
}

func (m *MockClient) GetResource(ctx context.Context, appName string, resource ResourceRef) (string, error) {
	_ = ctx
	for _, a := range m.apps {
		if a.Name != appName {
			continue
		}
		// A very rough live object.
		apiVersion := resource.Version
		if apiVersion == "" {
			if resource.Group != "" {
				apiVersion = resource.Group + "/v1"
			} else {
				apiVersion = "v1"
			}
		}
		ns := resource.Namespace
		metaNS := ""
		if ns != "" {
			metaNS = "\n  namespace: " + ns
		}
		return fmt.Sprintf("apiVersion: %s\nkind: %s\nmetadata:\n  name: %s%s\nspec: {}\nstatus: {}\n", apiVersion, resource.Kind, resource.Name, metaNS), nil
	}
	return "", fmt.Errorf("application not found: %s", appName)
}

func (m *MockClient) GetManifests(ctx context.Context, appName string) ([]string, error) {
	_ = ctx
	for _, a := range m.apps {
		if a.Name != appName {
			continue
		}
		out := make([]string, 0, len(a.Resources))
		for _, r := range a.Resources {
			ref := ResourceRef{Group: r.Group, Kind: r.Kind, Name: r.Name, Namespace: r.Namespace, Version: r.Version}
			man, _ := m.GetResource(ctx, appName, ref)
			out = append(out, man)
		}
		return out, nil
	}
	return nil, fmt.Errorf("application not found: %s", appName)
}

func (m *MockClient) ListEvents(ctx context.Context, appName string) ([]Event, error) {
	_ = ctx
	// Provide a tiny stable sample.
	for _, a := range m.apps {
		if a.Name == appName {
			return []Event{
				{Timestamp: time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339), Type: "Normal", Reason: "Synced", Message: "application synced", InvolvedObject: "Application/" + appName},
				{Timestamp: time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339), Type: "Warning", Reason: "Drift", Message: "resource out of sync detected", InvolvedObject: "Deployment/example"},
			}, nil
		}
	}
	return nil, fmt.Errorf("application not found: %s", appName)
}

func (m *MockClient) PodLogs(ctx context.Context, appName, podName, container string, follow bool) (io.ReadCloser, error) {
	_ = ctx
	_ = appName
	_ = podName
	_ = container
	_ = follow
	// Return a reader with a few sample lines. For follow, caller will just read until EOF.
	lines := []string{
		time.Now().Add(-3 * time.Second).UTC().Format(time.RFC3339) + " starting...",
		time.Now().Add(-2 * time.Second).UTC().Format(time.RFC3339) + " listening on :8080",
		time.Now().Add(-1 * time.Second).UTC().Format(time.RFC3339) + " GET /healthz 200",
	}
	return io.NopCloser(strings.NewReader(strings.Join(lines, "\n") + "\n")), nil
}

func (m *MockClient) ServerSideDiff(ctx context.Context, appName string) ([]DiffResult, error) {
	_ = ctx
	for _, a := range m.apps {
		if a.Name == appName {
			return []DiffResult{{
				Ref:      ResourceRef{Group: "apps", Kind: "Deployment", Name: appName, Namespace: a.Namespace, Version: "v1"},
				Modified: a.Sync != "Synced",
				Diff:     "--- live\n+++ desired\n@@\n- replicas: 1\n+ replicas: 2\n",
			}}, nil
		}
	}
	return nil, fmt.Errorf("application not found: %s", appName)
}

func (m *MockClient) RevisionMetadata(ctx context.Context, appName, revision string) (RevisionMeta, error) {
	_ = ctx
	_ = appName
	return RevisionMeta{Author: "alice", Date: "2026-02-01T12:34:56Z", Tags: []string{"v1.0.0"}, Message: "demo metadata for " + revision}, nil
}

func (m *MockClient) ChartDetails(ctx context.Context, appName, revision string) (ChartMeta, error) {
	_ = ctx
	_ = appName
	return ChartMeta{Description: "demo chart for " + revision, Maintainers: []string{"team-platform"}, Home: "https://example.com/charts"}, nil
}

func (m *MockClient) GetSyncWindows(ctx context.Context, appName string) ([]SyncWindow, error) {
	_ = ctx
	_ = appName
	return []SyncWindow{{Kind: "allow", Schedule: "* * * * *", Duration: "1h", Applications: []string{appName}, Namespaces: []string{"*"}}}, nil
}
