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
	RepoURL  string
	Revision string
	Path     string
	Cluster  string

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

	// RefreshApplication fetches an application, optionally forcing a cache bypass.
	// When hard is true, the server should refresh from source/cluster.
	RefreshApplication(ctx context.Context, name string, hard bool) (Application, error)

	// SyncApplication triggers an Argo CD sync operation.
	// When dryRun is true, the server should validate and simulate the operation without mutating state.
	SyncApplication(ctx context.Context, name string, dryRun bool) error
}
