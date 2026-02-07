package argocd

import (
	"context"
	"fmt"
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
				{Group: "apps", Kind: "Deployment", Name: "payments-api", Namespace: "payments", Status: "Synced", Health: "Healthy"},
				{Group: "", Kind: "Service", Name: "payments-api", Namespace: "payments", Status: "Synced", Health: "Healthy"},
				{Group: "", Kind: "ConfigMap", Name: "payments-config", Namespace: "payments", Status: "Synced", Health: "Healthy"},
				{Group: "autoscaling", Kind: "HorizontalPodAutoscaler", Name: "payments-api", Namespace: "payments", Status: "Synced", Health: "Healthy"},
			},
		},
		{
			Name:      "orders-worker",
			Namespace: "orders",
			Project:   "default",
			Health:    "Progressing",
			Sync:      "Synced",
			RepoURL:   "https://github.com/example/platform",
			Path:      "apps/orders",
			Revision:  "main",
			Cluster:   "https://kubernetes.default.svc",
			Resources: []Resource{
				{Group: "apps", Kind: "Deployment", Name: "orders-worker", Namespace: "orders", Status: "Synced", Health: "Progressing"},
				{Group: "batch", Kind: "CronJob", Name: "orders-reconciler", Namespace: "orders", Status: "Synced", Health: "Healthy"},
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
				{Group: "apps", Kind: "Deployment", Name: "web-frontend", Namespace: "web", Status: "OutOfSync", Health: "Healthy"},
				{Group: "", Kind: "Service", Name: "web-frontend", Namespace: "web", Status: "Synced", Health: "Healthy"},
				{Group: "networking.k8s.io", Kind: "Ingress", Name: "web", Namespace: "web", Status: "OutOfSync", Health: "Healthy"},
				{Group: "", Kind: "Secret", Name: "web-tls", Namespace: "web", Status: "OutOfSync", Health: "—"},
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
				{Group: "apps", Kind: "StatefulSet", Name: "loki", Namespace: "ops", Status: "Synced", Health: "Degraded"},
				{Group: "apps", Kind: "Deployment", Name: "grafana", Namespace: "ops", Status: "Synced", Health: "Healthy"},
				{Group: "", Kind: "Service", Name: "grafana", Namespace: "ops", Status: "Synced", Health: "Healthy"},
				{Group: "", Kind: "Job", Name: "migrate-dashboards", Namespace: "ops", Status: "Synced", Health: "Healthy", Hook: true},
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
				{Group: "apps", Kind: "DaemonSet", Name: "node-exporter", Namespace: "kube-system", Status: "Unknown", Health: "Missing"},
				{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "addons-read", Namespace: "", Status: "Unknown", Health: "—"},
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
