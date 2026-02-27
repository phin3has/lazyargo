package argocd

import (
	"context"
)

// VersionInfo mirrors /api/version.
type VersionInfo struct {
	Version    string `json:"Version"`
	BuildDate  string `json:"BuildDate"`
	GitCommit  string `json:"GitCommit"`
	GitTag     string `json:"GitTag"`
	GoVersion  string `json:"GoVersion"`
	Platform   string `json:"Platform"`
	Helm       string `json:"HelmVersion"`
	Kubectl    string `json:"KubectlVersion"`
	Kustomize  string `json:"KustomizeVersion"`
	Jsonnet    string `json:"JsonnetVersion"`
	Compiler   string `json:"Compiler"`
	TreeState  string `json:"GitTreeState"`
}

func (c *HTTPClient) GetVersion(ctx context.Context) (VersionInfo, error) {
	var out VersionInfo
	if err := c.doJSON(ctx, "GET", "/api/version", nil, &out); err != nil {
		return VersionInfo{}, err
	}
	return out, nil
}
