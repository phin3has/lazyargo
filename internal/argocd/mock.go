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
	_ = ctx
	for _, a := range m.apps {
		if a.Name == name {
			return a, nil
		}
	}
	return Application{}, fmt.Errorf("application not found: %s", name)
}
