package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	entire "go.entire.com/entire-sdk-go"
)

// Limits on a deployments upsert.
const (
	// MaxDeployments is the most deployments one upsert may carry.
	MaxDeployments = 1000
	// MaxDeploymentText bounds a deployment's Key and Name, and its
	// group's Key and Name, in characters.
	MaxDeploymentText = 255
	// MaxDeploymentLabel bounds a deployment's Type and Environment, in
	// characters.
	MaxDeploymentLabel = 64
	// MaxDeploymentURLBytes bounds EnvironmentURL and LogsURL.
	MaxDeploymentURLBytes = 2048
)

// Inactive is the status of a deployment whose resource no longer exists,
// such as a torn-down preview. Only deployments take it; check runs do not.
const Inactive Status = "inactive"

// Preview is the default environment, and the only one Entire documents.
const Preview = "preview"

// DeploymentGroup gathers related deployments, such as the site and
// functions of one project, under one heading on the Change.
type DeploymentGroup struct {
	// Key identifies the group, at most 255 characters.
	Key string `json:"key"`
	// Name is the heading shown, at most 255 characters.
	Name string `json:"name"`
}

// Deployment is a deployed resource the plugin reports on a commit, such as
// a preview site. Entire shows it, with its links, on the Change; it never
// gates the merge.
type Deployment struct {
	// Key identifies the resource, at most 255 characters. Keep it stable
	// across updates and commits. Keys belong to the plugin: another
	// plugin's deployment with the same key is a different resource.
	Key string
	// Name is shown on the Change, at most 255 characters.
	Name string
	// Type is a free-form kind, such as "site" or "function", at most 64
	// characters. Optional.
	Type string
	// Group, optional, gathers the deployment with others under a heading.
	Group *DeploymentGroup
	// Environment is where the resource runs, at most 64 characters.
	// Empty means Preview. Other values are stored and returned, but
	// Entire treats every environment the same way.
	Environment string
	// Status is the deployment's progress: Queued, then InProgress, then
	// Completed. Report Inactive when the resource is removed, so that its
	// links stop being offered.
	Status Status
	// Conclusion is the result, required when Status is Completed and empty
	// otherwise: Success, Failure, Cancelled, or Skipped.
	Conclusion Conclusion
	// EnvironmentURL links to the deployed resource. It must be https, with
	// no credentials. Optional.
	EnvironmentURL string
	// LogsURL links to the deployment's logs. It must be https, with no
	// credentials. Optional.
	LogsURL string
	// ExternalUpdatedAt, required, is when this state happened on the
	// plugin's side, from the plugin's own records rather than the time of
	// the call. Writes apply in its order, so an older report that arrives
	// late cannot overwrite newer state. Reuse it unchanged when retrying.
	ExternalUpdatedAt time.Time
	// StartedAt is when the deployment started, if it has.
	StartedAt time.Time
	// CompletedAt is when the deployment completed, if it has.
	CompletedAt time.Time
	// DeadlineAt, at most 24 hours ahead, is when a queued or in-progress
	// deployment is shown as stale.
	DeadlineAt time.Time
}

// MarshalJSON encodes the deployment as the API expects, with times in UTC
// to the millisecond and zero times left out.
func (d Deployment) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Key               string           `json:"key"`
		Name              string           `json:"name"`
		Type              string           `json:"type,omitempty"`
		Group             *DeploymentGroup `json:"group,omitempty"`
		Environment       string           `json:"environment,omitempty"`
		Status            Status           `json:"status"`
		Conclusion        Conclusion       `json:"conclusion,omitempty"`
		EnvironmentURL    string           `json:"environment_url,omitempty"`
		LogsURL           string           `json:"logs_url,omitempty"`
		ExternalUpdatedAt string           `json:"external_updated_at"`
		StartedAt         string           `json:"started_at,omitempty"`
		CompletedAt       string           `json:"completed_at,omitempty"`
		DeadlineAt        string           `json:"deadline_at,omitempty"`
	}{d.Key, d.Name, d.Type, d.Group, d.Environment, d.Status, d.Conclusion, d.EnvironmentURL, d.LogsURL,
		apiTime(d.ExternalUpdatedAt), optionalTime(d.StartedAt), optionalTime(d.CompletedAt), optionalTime(d.DeadlineAt)})
}

// DeploymentsResult is the stored state after an upsert.
type DeploymentsResult struct {
	// Results has one entry per deployment, in request order.
	Results []DeploymentResult `json:"results"`
}

// DeploymentResult is what an upsert did to one deployment.
type DeploymentResult struct {
	// Deployment is the stored deployment, whether or not this write
	// applied.
	Deployment DeploymentRef `json:"deployment"`
	// Outcome says whether this write applied.
	Outcome Outcome `json:"outcome"`
}

// DeploymentRef is a stored deployment's identity and state.
type DeploymentRef struct {
	// ID is Entire's ID for the deployment.
	ID string `json:"id"`
	// Key is the deployment's key.
	Key string `json:"key"`
	// Status is the stored status.
	Status Status `json:"status"`
	// Conclusion is the stored conclusion, empty unless completed.
	Conclusion Conclusion `json:"conclusion"`
}

// StoredDeployment is the plugin's current state of one deployment on a
// commit. Times the plugin never sent are zero.
type StoredDeployment struct {
	// ID is Entire's ID for the deployment.
	ID string `json:"id"`
	// Plugin is the slug of the plugin that reported it.
	Plugin            string           `json:"plugin"`
	Key               string           `json:"key"`
	Name              string           `json:"name"`
	Type              string           `json:"type"`
	Group             *DeploymentGroup `json:"group"`
	Environment       string           `json:"environment"`
	Status            Status           `json:"status"`
	Conclusion        Conclusion       `json:"conclusion"`
	EnvironmentURL    string           `json:"environment_url"`
	LogsURL           string           `json:"logs_url"`
	ExternalUpdatedAt entire.Timestamp `json:"external_updated_at"`
	StartedAt         entire.Timestamp `json:"started_at"`
	CompletedAt       entire.Timestamp `json:"completed_at"`
	DeadlineAt        entire.Timestamp `json:"deadline_at"`
}

// Deployments reports the plugin's deployed resources, with their links, on
// a commit's Change. It reports deployments; it does not create them, and
// they never gate the merge, so a plugin that gates merges keeps writing
// check runs as well. Every deployment belongs to the plugin that wrote it:
// other plugins can neither read nor change it.
//
// Writes to a deployment apply in ExternalUpdatedAt order, so retrying a
// write is safe and an older report never overwrites a newer one. At an
// equal time a write never moves a deployment back to an earlier status, in
// the order Queued, InProgress, Completed, Inactive, so a late retry cannot
// revive a torn-down preview.
//
// Deployments needs Permissions.Deployments: Write, which each project must
// accept before it takes effect there. Until a project accepts it, calls
// for the project's repositories answer 403 with entire.CodeForbidden; keep
// reporting through check runs and discussions there. Each call uses a token
// for its repository that carries only that permission.
type Deployments struct{ inst *Installation }

var deploymentsWrite = Permissions{Deployments: Write}

// DeploymentsRequest is one deployments write: deployments on a commit of a
// branch.
type DeploymentsRequest struct {
	// Branch is the branch the commit was deployed from, such as
	// "feature-x". A leading "refs/heads/" is removed.
	Branch string
	// Deployments are the deployments to write: 1 to 1,000, with no two
	// sharing a key. The request body may be at most 1 MiB.
	Deployments []Deployment
}

// Upsert writes deployments on a commit, whenever their state changes. It
// is idempotent, and a stale write returns the stored deployment with the
// outcome IgnoredStale.
//
// The SDK checks the write as the API does before sending it, sends it to
// the repository's processing primary, retries failures with the same body,
// and when Entire answers that the repository moved (wrong_cell), lists the
// repositories again and retries at the new host. repoID is a public
// repository ID, such as Repository.ID.
func (d *Deployments) Upsert(ctx context.Context, repoID, sha string, req DeploymentsRequest) (*entire.Response[DeploymentsResult], error) {
	branch := strings.TrimPrefix(req.Branch, "refs/heads/")
	if err := validateDeployments(repoID, sha, branch, req.Deployments, d.inst.app.now()); err != nil {
		return nil, err
	}
	body := struct {
		Branch      string       `json:"branch"`
		Deployments []Deployment `json:"deployments"`
	}{branch, req.Deployments}
	return send[DeploymentsResult](ctx, d.inst, "v0", repoID, deploymentsWrite, func(client *entire.Client) *entire.Request {
		r := client.NewRequest(http.MethodPost, checkPath(repoID, sha, "deployments"), body)
		// The upsert is idempotent, so a failed attempt may be retried.
		r.Idempotent = true
		return r
	})
}

// List returns one page of the plugin's current deployments on a commit,
// one per key, ordered by key. opts.PageSize is 1 to 100, 30 when zero;
// Response.Next is the PageToken of the following page, empty on the last.
func (d *Deployments) List(ctx context.Context, repoID, sha string, opts *entire.ListOptions) (*entire.Response[[]StoredDeployment], error) {
	if err := validateCommit(repoID, sha); err != nil {
		return nil, err
	}
	type envelope struct {
		Deployments []StoredDeployment `json:"deployments"`
		NextCursor  string             `json:"next_cursor"`
	}
	resp, err := send[envelope](ctx, d.inst, "v0", repoID, deploymentsWrite, func(client *entire.Client) *entire.Request {
		r := client.NewRequest(http.MethodGet, checkPath(repoID, sha, "deployments"), nil)
		if opts != nil && opts.PageSize > 0 {
			r.Query.Set("per_page", strconv.Itoa(opts.PageSize))
		}
		if opts != nil && opts.PageToken != "" {
			r.Query.Set("cursor", opts.PageToken)
		}
		return r
	})
	out, err := unwrap(resp, err, func(v envelope) []StoredDeployment { return v.Deployments })
	if out != nil {
		out.Next = resp.Data.NextCursor
	}
	return out, err
}

// All iterates over every one of the plugin's current deployments on a
// commit, fetching pages as it goes. Each range starts from the first page.
func (d *Deployments) All(ctx context.Context, repoID, sha string) iter.Seq2[StoredDeployment, error] {
	return entire.Iterate(func(next string) (*entire.Response[[]StoredDeployment], error) {
		return d.List(ctx, repoID, sha, &entire.ListOptions{PageToken: next})
	}, func(resp *entire.Response[[]StoredDeployment]) []StoredDeployment { return resp.Data })
}

func validateDeployments(repoID, sha, branch string, deployments []Deployment, now time.Time) error {
	if err := validateCommit(repoID, sha); err != nil {
		return err
	}
	if strings.TrimSpace(branch) == "" {
		return errors.New("plugin: branch is required")
	}
	if strings.ContainsFunc(branch, unicode.IsControl) {
		return errors.New("plugin: branch must not contain control characters")
	}
	if len(deployments) == 0 || len(deployments) > MaxDeployments {
		return fmt.Errorf("plugin: an upsert needs 1 to %d deployments, not %d", MaxDeployments, len(deployments))
	}
	seen := make(map[string]bool, len(deployments))
	for n, d := range deployments {
		fail := func(format string, args ...any) error {
			return fmt.Errorf("plugin: deployment %d (%q): %s", n, d.Key, fmt.Sprintf(format, args...))
		}
		switch {
		case strings.TrimSpace(d.Key) == "":
			return fail("key is required")
		case strings.TrimSpace(d.Name) == "":
			return fail("name is required")
		case seen[d.Key]:
			return fail("repeats a key")
		case d.Group != nil && (strings.TrimSpace(d.Group.Key) == "" || strings.TrimSpace(d.Group.Name) == ""):
			return fail("a group needs a key and a name")
		}
		seen[d.Key] = true
		texts := []deploymentText{{"key", d.Key, MaxDeploymentText}, {"name", d.Name, MaxDeploymentText},
			{"type", d.Type, MaxDeploymentLabel}, {"environment", d.Environment, MaxDeploymentLabel}}
		if d.Group != nil {
			texts = append(texts, deploymentText{"group key", d.Group.Key, MaxDeploymentText},
				deploymentText{"group name", d.Group.Name, MaxDeploymentText})
		}
		for _, t := range texts {
			if utf8.RuneCountInString(t.value) > t.max {
				return fail("%s exceeds %d characters", t.name, t.max)
			}
			if strings.ContainsFunc(t.value, unicode.IsControl) {
				return fail("%s contains a control character", t.name)
			}
		}
		switch {
		case d.Status != Queued && d.Status != InProgress && d.Status != Completed && d.Status != Inactive:
			return fail("status %q is not queued, in_progress, completed, or inactive", d.Status)
		case d.Status == Completed && d.Conclusion == "":
			return fail("a completed deployment needs a conclusion")
		case d.Status == Completed && d.Conclusion != Success && d.Conclusion != Failure && d.Conclusion != Cancelled && d.Conclusion != Skipped:
			return fail("conclusion %q is not success, failure, cancelled, or skipped", d.Conclusion)
		case d.Status != Completed && d.Conclusion != "":
			return fail("only a completed deployment has a conclusion")
		case d.ExternalUpdatedAt.IsZero():
			return fail("external updated time is required; set it from the plugin's own records")
		case !d.StartedAt.IsZero() && !d.CompletedAt.IsZero() && d.CompletedAt.Before(d.StartedAt):
			return fail("completed before it started")
		case !d.DeadlineAt.IsZero() && d.DeadlineAt.After(now.Add(MaxDeadline)):
			return fail("deadline is more than 24 hours ahead")
		}
		for _, u := range []struct{ name, value string }{{"environment URL", d.EnvironmentURL}, {"logs URL", d.LogsURL}} {
			if err := validateDeploymentURL(u.value); err != nil {
				return fail("%s: %v", u.name, err)
			}
		}
	}
	return nil
}

// deploymentText is a text field of a deployment and its limit in
// characters.
type deploymentText struct {
	name, value string
	max         int
}

func validateDeploymentURL(raw string) error {
	if raw == "" {
		return nil
	}
	if len(raw) > MaxDeploymentURLBytes {
		return fmt.Errorf("exceeds %d bytes", MaxDeploymentURLBytes)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("%q is not an https URL without credentials", raw)
	}
	return nil
}
