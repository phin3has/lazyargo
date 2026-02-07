package argocd

// ResourceRef identifies a specific resource instance in an application.
// It is used for endpoints that require group/kind/version/name/namespace.
type ResourceRef struct {
	Group     string
	Kind      string
	Name      string
	Namespace string
	Version   string
}

type DiffResult struct {
	Ref      ResourceRef
	Diff     string
	Modified bool
}

type Event struct {
	Type           string
	Reason         string
	Message        string
	Timestamp      string
	InvolvedObject string
}

type SyncHistoryEntry struct {
	Revision   string
	DeployedAt string
	Status     string
	Message    string
	Source     string
}

type SyncWindow struct {
	Kind         string
	Schedule     string
	Duration     string
	Applications []string
	Namespaces   []string
}

type AppCondition struct {
	Type    string
	Message string
}

type RevisionMeta struct {
	Author  string
	Date    string
	Tags    []string
	Message string
}

type ChartMeta struct {
	Description string
	Maintainers []string
	Home        string
}
