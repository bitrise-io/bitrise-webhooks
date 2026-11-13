package plugin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	entire "go.entire.com/entire-sdk-go"
)

// ErrRepositoryExists reports a create that Core refused with 409 Conflict
// and the repo/name-in-use problem type: the project already has a
// repository with that name. The error also wraps
// the *entire.HTTPError.
var ErrRepositoryExists = errors.New("plugin: a repository with that name already exists in the project")

// ErrCreateLimit reports a create that Core refused with 429 Too Many
// Requests and the repo/create-limit-reached problem type: the installation
// reached its limit on repository creates, by default 100 in any 24 hours. The error also wraps the *entire.HTTPError,
// whose RetryAfter says when to try again.
var ErrCreateLimit = errors.New("plugin: the installation reached its repository create limit")

// Readiness polling of a created repository. Variables, so that tests can
// shorten them.
var (
	// readyPollInitial is the first wait between repository lists.
	readyPollInitial = 500 * time.Millisecond
	// readyPollMax caps the wait between repository lists.
	readyPollMax = 5 * time.Second
	// readyTimeout bounds the wait for a created repository to be ready.
	readyTimeout = 2 * time.Minute
)

// CreateRepositoryOptions are the optional settings of a new repository.
type CreateRepositoryOptions struct {
	// ObjectFormat is the repository's Git hash, "sha1" or "sha256". Empty
	// selects "sha1".
	ObjectFormat string
	// ProjectID is the ID of the project to create the repository in. Empty
	// selects the installation's project, the only one Core accepts: it
	// answers 404 Not Found for another.
	ProjectID string
}

// createRepositoryRequest is the body of a repository create.
type createRepositoryRequest struct {
	Name         string `json:"name"`
	ObjectFormat string `json:"objectFormat,omitempty"`
	ProjectID    string `json:"projectId,omitempty"`
}

// createdRepository is the part of Core's created repository that the SDK
// reads.
type createdRepository struct {
	// RepositoryID is the installation's ID of the repository, repo_….
	RepositoryID string `json:"repositoryId"`
	Name         string `json:"name"`
	State        string `json:"state"`
}

// CreateRepository creates a repository with the given name in the
// installation's project, and returns it once it is ready: once the
// installation's repository list holds it with a processing primary, so
// that RepositoryToken, RepositoryClient, GitEnv, and check runs work on
// it. It needs Repositories: Create in Config.Permissions:
//
//	repo, err := inst.CreateRepository(ctx, "widget", nil)
//	client, repoID, err := inst.RepositoryClient(ctx, repo.ID, plugin.Permissions{Contents: plugin.Write})
//
// The create uses a token that carries only repositories:create, cached and
// renewed as RepositoryToken's are. Core adds the repository to the
// installation, even one limited to selected repositories, and sends the
// plugin an installation.repositories_added event.
//
// A create that Core refuses returns ErrRepositoryExists for a name the
// project already has, and ErrCreateLimit when the installation has created
// too many repositories; both wrap the *entire.HTTPError too. Other refusals
// return the *entire.HTTPError: 400 for an invalid name or object format,
// 403 for an installation that is suspended or lacks repositories:create.
// A create is never retried, since a retry after an unanswered create would
// fail with ErrRepositoryExists.
//
// CreateRepository waits up to two minutes, and no longer than ctx allows,
// for a repository that Core is still provisioning. When the wait ends
// first, it returns ErrRepositoryNotReady, or the failure of the last
// repository list, with a Repository that holds the new repository's ID:
// the repository exists, so look for it again later with Repository rather
// than creating it again.
func (i *Installation) CreateRepository(ctx context.Context, name string, opts *CreateRepositoryOptions) (Repository, error) {
	if name == "" {
		return Repository{}, errors.New("plugin: repository name is required")
	}
	if i.app.cfg.Permissions.Repositories != Create {
		return Repository{}, errors.New("plugin: creating a repository needs Repositories: Create in Config.Permissions")
	}
	body := createRepositoryRequest{Name: name}
	if opts != nil {
		body.ObjectFormat, body.ProjectID = opts.ObjectFormat, opts.ProjectID
	}
	token, err := i.token(ctx, tokenRequest{Permissions: Permissions{Repositories: Create}})
	if err != nil {
		return Repository{}, err
	}
	r := i.coreClient.NewRequest(http.MethodPost, "/repos", body)
	r.Core = true
	resp, err := entire.Call[createdRepository](ctx, i.coreClient, r, entire.WithHeader("Authorization", "Bearer "+token.Value))
	switch {
	case problemType(err) == problemRepoNameInUse:
		return Repository{}, fmt.Errorf("%w: %q: %w", ErrRepositoryExists, name, err)
	case problemType(err) == problemRepoCreateLimitReached:
		return Repository{}, fmt.Errorf("%w: %w", ErrCreateLimit, err)
	case err != nil:
		return Repository{}, fmt.Errorf("plugin: create repository %q in installation %s: %w", name, i.id, err)
	}
	id := resp.Data.RepositoryID
	if id == "" {
		return Repository{}, fmt.Errorf("plugin: create repository %q in installation %s: response has no repositoryId", name, i.id)
	}
	// The cached list predates the repository.
	i.forgetRepositories()
	return i.awaitRepository(ctx, id)
}

// forgetRepositories expires the cached repository list, so that the next
// Repository call lists the repositories again.
func (i *Installation) forgetRepositories() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.listedAt = time.Time{}
}

// awaitRepository lists the installation's repositories, with growing
// waits, until the repository with the given ID has a processing primary.
func (i *Installation) awaitRepository(ctx context.Context, id string) (Repository, error) {
	ctx, cancel := context.WithTimeout(ctx, readyTimeout)
	defer cancel()
	pending := Repository{ID: id}
	notReady := fmt.Errorf("%w: %s", ErrRepositoryNotReady, id)
	last := notReady
	wait := readyPollInitial
	for {
		repos, err := i.Repositories(ctx)
		switch {
		case err == nil:
			last = notReady
			for _, r := range repos {
				if r.ID != id {
					continue
				}
				if r.PrimaryAPIURL() != "" {
					return r, nil
				}
				pending = r
			}
		case ctx.Err() == nil:
			last = err
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			// The last list may hold the repository without a
			// processing primary; let Repository list again.
			i.forgetRepositories()
			return pending, last
		case <-timer.C:
		}
		wait = min(2*wait, readyPollMax)
	}
}

// Core names these refusals by RFC 9457 problem type. A status alone is too
// coarse: 409 and 429 also carry other conditions.
const (
	problemRepoNameInUse          = "https://entire.com/errors/repo/name-in-use"
	problemRepoCreateLimitReached = "https://entire.com/errors/repo/create-limit-reached"
)

// problemType returns the problem type URI of a Core refusal, or "".
func problemType(err error) string {
	var httpErr *entire.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Type
	}
	return ""
}
