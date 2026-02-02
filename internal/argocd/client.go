package argocd

import "context"

// Application is a minimal representation of an Argo CD application.
// Expand as the UI needs more information.
type Application struct {
	Name      string
	Namespace string
	Project   string
	Status    string // e.g. Healthy, Degraded
	Sync      string // e.g. Synced, OutOfSync
}

// Client is the interface the UI depends on.
//
// Keep it narrow: the UI shouldn't know about transport/proto details.
type Client interface {
	ListApplications(ctx context.Context) ([]Application, error)
	GetApplication(ctx context.Context, name string) (Application, error)
}
