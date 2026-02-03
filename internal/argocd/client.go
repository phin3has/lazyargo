package argocd

import "context"

// Application is a minimal representation of an Argo CD application.
// Expand as the UI needs more information.
type Application struct {
	Name      string
	Namespace string
	Project   string
	Health    string // e.g. Healthy, Degraded
	Sync      string // e.g. Synced, OutOfSync

	// Optional fields (may be empty depending on API permissions / list endpoint)
	RepoURL   string
	Revision  string
	Path      string
	Cluster   string

	// Resources are usually populated by GetApplication.
	Resources []Resource
}

type Resource struct {
	Group     string
	Kind      string
	Name      string
	Namespace string
	Status    string
	Health    string
	Hook      bool
}

// Client is the interface the UI depends on.
//
// Keep it narrow: the UI shouldn't know about transport/proto details.
type Client interface {
	ListApplications(ctx context.Context) ([]Application, error)
	GetApplication(ctx context.Context, name string) (Application, error)
}
