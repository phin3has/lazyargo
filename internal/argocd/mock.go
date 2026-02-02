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
		{Name: "payments-api", Namespace: "payments", Project: "default", Status: "Healthy", Sync: "Synced"},
		{Name: "web-frontend", Namespace: "web", Project: "default", Status: "Healthy", Sync: "OutOfSync"},
		{Name: "observability", Namespace: "ops", Project: "platform", Status: "Degraded", Sync: "Synced"},
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
