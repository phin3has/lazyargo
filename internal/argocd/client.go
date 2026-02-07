package argocd

import (
	"context"
	"io"
)

// Application is a minimal representation of an Argo CD application.
// Expand as the UI needs more information.
type Application struct {
	Name      string
	Namespace string
	Project   string
	Health    string // e.g. Healthy, Degraded
	Sync      string // e.g. Synced, OutOfSync

	OperationState *OperationState

	SyncPolicy string // e.g. auto/manual

	// Optional fields (may be empty depending on API permissions / list endpoint)
	RepoURL  string
	Revision string
	Path     string
	Cluster  string

	// Resources are usually populated by GetApplication.
	Resources []Resource

	// History is populated by Get/Refresh when available.
	History []SyncHistoryEntry
}

type OperationState struct {
	Phase   string
	Message string
}

type Revision struct {
	ID       int64
	Revision string
	Author   string
	Date     string
	Message  string
}

type Resource struct {
	Group     string
	Kind      string
	Version   string
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

	ListRevisions(ctx context.Context, name string) ([]Revision, error)
	RollbackApplication(ctx context.Context, name string, revisionID int64) error
	TerminateOperation(ctx context.Context, name string) error
	DeleteApplication(ctx context.Context, name string, cascade bool) error
	CreateApplication(ctx context.Context, app Application) error
	ListProjects(ctx context.Context) ([]string, error)
	ListClusters(ctx context.Context) ([]string, error)
	ListRepositories(ctx context.Context) ([]string, error)
	UpdateApplication(ctx context.Context, app Application) error

	// SyncApplication triggers an Argo CD sync operation.
	// When dryRun is true, the server should validate and simulate the operation without mutating state.
	SyncApplication(ctx context.Context, name string, dryRun bool) error

	// Phase 2 additions.
	GetResource(ctx context.Context, appName string, resource ResourceRef) (string, error)
	GetManifests(ctx context.Context, appName string) ([]string, error)
	ListEvents(ctx context.Context, appName string) ([]Event, error)
	PodLogs(ctx context.Context, appName, podName, container string, follow bool) (io.ReadCloser, error)
	ServerSideDiff(ctx context.Context, appName string) ([]DiffResult, error)
	RevisionMetadata(ctx context.Context, appName, revision string) (RevisionMeta, error)
	ChartDetails(ctx context.Context, appName, revision string) (ChartMeta, error)
	GetSyncWindows(ctx context.Context, appName string) ([]SyncWindow, error)
}
