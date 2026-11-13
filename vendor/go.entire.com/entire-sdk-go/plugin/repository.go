package plugin

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	entire "go.entire.com/entire-sdk-go"
)

// RepositoryClient returns an SDK client that acts for the installation on
// one of its repositories, and the ID that the client's repository methods
// take:
//
//	client, repoID, err := inst.RepositoryClient(ctx, repo.ID, plugin.Permissions{Contents: plugin.Read})
//	branches, err := client.Git.ListBranches(ctx, repoID, nil)
//
// The client sends RepositoryToken's narrowed token, and renews it. Every
// request goes to the repository's processing primary: Core does not accept
// installation tokens, so the client's Core methods fail, and so do
// repository methods for other repositories.
//
// The returned ID is the repository's ID in its region, which differs from
// the installation's repository ID. Only the client accepts it.
//
// RepositoryClient makes requests: it lists the installation's repositories
// unless Repository has them cached, mints the token unless RepositoryToken
// has it cached, and asks the region for the repository's ID.
func (i *Installation) RepositoryClient(ctx context.Context, repoID string, permissions Permissions) (*entire.Client, string, error) {
	repo, err := i.Repository(ctx, repoID)
	if err != nil {
		return nil, "", err
	}
	apiURL := repo.PrimaryAPIURL()
	if apiURL == "" {
		return nil, "", fmt.Errorf("%w: %s", ErrRepositoryNotReady, repoID)
	}
	origin, err := i.app.checkOrigin(apiURL)
	if err != nil {
		return nil, "", err
	}
	source := func(ctx context.Context) (string, error) {
		token, err := i.RepositoryToken(ctx, repoID, permissions)
		if err != nil {
			return "", err
		}
		return token.Value, nil
	}
	client, err := entire.NewClient("", i.app.clientOptions(entire.WithBaseURL(origin.String()), entire.WithTokenSource(source))...)
	if err != nil {
		return nil, "", err
	}
	// The region knows the repository by another ID, which most repository
	// methods require. It answers for either.
	resp, err := entire.Call[struct {
		ID string `json:"id"`
	}](ctx, client, client.NewRequest(http.MethodGet, "/repos/"+url.PathEscape(repoID), nil))
	if err != nil {
		return nil, "", fmt.Errorf("plugin: look up repository %s: %w", repoID, err)
	}
	if resp.Data.ID == "" {
		return nil, "", fmt.Errorf("plugin: repository %s has no ID in its region", repoID)
	}
	return client, resp.Data.ID, nil
}
