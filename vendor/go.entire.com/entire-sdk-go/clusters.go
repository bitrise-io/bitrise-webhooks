package entire

import (
	"context"
	"net/http"
)

// ClustersService lists the clusters that host repositories. The listing is
// experimental: Entire may change it without notice.
type ClustersService service

// Cluster is a cluster: a deployment that hosts repositories' Git data in
// one region.
type Cluster struct {
	// Slug names the cluster, such as "aws-us-east-2".
	Slug string `json:"slug"`
	// Jurisdiction is the cluster's region, such as "us".
	Jurisdiction string `json:"jurisdiction"`
	// PublicURL is the base URL of the cluster's Git service.
	PublicURL string `json:"publicUrl"`
	// IsDefault reports the default cluster of its region.
	IsDefault bool `json:"isDefault"`
	// APIURL is the base URL of the Entire API of the cluster's cell, which
	// is distinct from PublicURL. It is empty when Core cannot route to the
	// cell.
	APIURL string `json:"apiUrl"`
}

// List returns the clusters of every region. The answer is the same for
// every signed-in caller; an anonymous client fails with 401 Unauthorized.
func (s *ClustersService) List(ctx context.Context, reqOpts ...RequestOption) (*Response[[]Cluster], error) {
	type envelope struct {
		Clusters []Cluster `json:"clusters"`
	}
	return unwrap(ctx, s.client, coreRequest(http.MethodGet, nil, "/clusters"), reqOpts, func(e *envelope) []Cluster { return e.Clusters })
}
