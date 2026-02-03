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
		{Name: "payments-api", Namespace: "payments", Project: "default", Health: "Healthy", Sync: "Synced", RepoURL: "https://github.com/example/platform", Path: "apps/payments", Revision: "main", Cluster: "https://kubernetes.default.svc"},
		{Name: "web-frontend", Namespace: "web", Project: "default", Health: "Healthy", Sync: "OutOfSync", RepoURL: "https://github.com/example/platform", Path: "apps/web", Revision: "main", Cluster: "https://kubernetes.default.svc"},
		{Name: "observability", Namespace: "ops", Project: "platform", Health: "Degraded", Sync: "Synced", RepoURL: "https://github.com/example/ops", Path: "apps/observability", Revision: "main", Cluster: "https://kubernetes.default.svc"},
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
