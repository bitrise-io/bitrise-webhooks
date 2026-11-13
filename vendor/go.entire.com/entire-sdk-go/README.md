# Entire Go SDK

[![Go Docs](https://img.shields.io/badge/Go-Docs-007D9C?logo=go&logoColor=white)](https://pkg.go.dev/go.entire.com/entire-sdk-go)

Go client for Entire: repositories and Git, Trails, Changes, and code review, the agent sessions and checkpoints behind your commits, search, and the organizations and projects around them, all through one client. Standard library only; Go 1.24 or newer.

## Quick start

```sh
go get go.entire.com/entire-sdk-go
export ENTIRE_TOKEN="$(entire auth token)"  # your Entire CLI login
```

This program prints a repository's ten latest commits:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	entire "go.entire.com/entire-sdk-go"
)

func main() {
	client, err := entire.NewClientFromEnv() // reads ENTIRE_TOKEN
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Your repository: gh/owner/repo on GitHub, or et/project/repo on Entire.
	repoID, err := client.ResolveRepoID(ctx, "gh/acme/widget")
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.Git.ListCommits(ctx, repoID, "HEAD", &entire.CommitListOptions{
		ListOptions: entire.ListOptions{PageSize: 10},
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, commit := range resp.Data {
		subject, _, _ := strings.Cut(commit.Message, "\n")
		fmt.Println(commit.SHA[:7], subject)
	}
}
```

The [examples](examples/README.md) are complete programs that list your repositories and projects, resolve a repository and its Changes, watch a Change live, and run a self-hosted agent runner.

## Services

The client groups the API into services, one per resource:

| Service | Covers |
| --- | --- |
| `Trails` | Project Trails: their intent, their Changes, their timelines, and their discussions |
| `Changes` | A repository's Changes: listing, creating, and updating them, their events, approvals, gates, and merges, code reviews and their comments, and discussions |
| `Git` | Branches, tags, and refs, commits, comparisons, merge bases, trees, files, and diffs, branch creation, and live ref changes |
| `Checkpoints`, `Sessions` | The agent sessions behind commits (`Checkpoints.ListForCommit` maps a commit to them), and their transcripts |
| `Search` | Code search, and semantic search over repositories, checkpoints, commits, pull requests, and sessions |
| `Repos` | The repository directory and repositories, the agents that worked on a repository's code, and the CI checks of its commits |
| `Me`, `Orgs`, `Projects` | Your account, your project Trails, and your sessions across repositories; organizations and their members; projects and their repositories |
| `Clusters` | The clusters that host repositories, in every region (experimental) |
| `ServiceAccounts` | A project's service accounts, their grants, OIDC bindings, and secrets, and the OIDC providers a binding can trust |
| `Agents`, `Environments`, `Runs` | A project's review agents, their versions and history, and Entire's model, price, and preset catalog; the Environments they run in and the runners bound to them; starting, reading, retrying, and canceling their runs. Experimental: not yet part of Entire's published API, and may change without a major SDK release |
| `Runner` | The self-hosted runner loop: list, claim, complete, and fail runs of self-hosted agents with a service account. Experimental, as above |

Every request goes where its data lives, so you never choose an endpoint. The SDK covers what developers building on Entire need first and grows as they need more; the rest of the API is a few lines away (see [Endpoints the SDK doesn't cover](#endpoints-the-sdk-doesnt-cover)).

## Calling a method

Methods take a context, then the path arguments, such as a repository ID and a Change number, then an options struct, then the request body, and finally request options:

```go
change, err := client.Changes.Get(ctx, repoID, 42, nil)
created, err := client.Changes.Create(ctx, repoID, entire.ChangeCreateRequest{Title: "Speed up the review page", Branch: "speed"})
branches, err := client.Git.ListBranches(ctx, repoID, &entire.RefSearchOptions{Search: "feature/"})
```

JSON methods return `*entire.Response[T]`, with the decoded result in `Data` and the status code and headers alongside.

- **Options** are a pointer to an `…Options` struct, or nil for the defaults. **Bodies** are values: a `…Request` struct, or plain arguments such as a message's text.
- **Types** are plain structs that mirror the API. A field is a pointer only where nil means something, such as `Change.Mergeability`, which only `Changes.Get` sets. Times are `entire.Timestamp`, which embeds `time.Time` and is zero when the API sent none.
- **Updates** use pointers for the fields to change: `entire.Ptr(v)` makes one. Where a field can be cleared, it is an `entire.Nullable[T]`: `entire.Value(v)` sets it, and `entire.Null[T]()` clears it.
- **Enums** that you filter or set on are typed constants, such as `entire.ChangeStatusOpen`. Values the SDK doesn't know yet still decode.
- **Files and transcripts** can be read whole, with `Git.ReadFile`, or streamed, with `Git.OpenFile` and `Checkpoints.RawTranscript`; close what you stream. To see the raw bytes of a JSON response as well, pass `entire.WithResponseBody(w)`.

## Trails and Changes

A Trail belongs to a project, and holds the work's title, description, type, priority, and assignees. Each repository branch of the Trail is a Change, with its own number in its repository. Open a Trail with its branches in one call:

```go
trail, err := client.Trails.CreateInProject(ctx, "et/acme", entire.ProjectTrailCreateRequest{
	Title: "Bump dependencies",
	Body:  "Updates vision and mosaic.",
	Changes: []entire.ProjectTrailChangeRequest{{
		RepositoryID: repoID, BranchName: "deps", BranchAction: "link",
	}},
})
```

`ListInProject`, `GetInProject`, and `UpdateInProject` read and change a Trail, `AddChange`, `GetChange`, and `AttachChange` manage its Changes, and `ListTimeline` and the discussion methods follow its history and conversation. `Me.ListProjectTrails` lists the Trails you opened or are assigned to. The `Changes` service acts on one Change, by repository ID and the Change's number (`ProjectTrailChange.Number`): reading it, its gates, reviews, and discussions, and merging it. A repository with Trails turned off in its settings refuses them all. `CreateInProject` with a Change, `AddChange`, and `ListInProject` with `RepoID` find the repository rather than the project, so a service account granted only that repository can open and list its Trails.

## Pagination

A list method returns one page, and its response's `Next` continues the listing:

```go
opts := &entire.RefSearchOptions{ListOptions: entire.ListOptions{PageSize: 100}}
for {
	resp, err := client.Git.ListBranches(ctx, repoID, opts)
	if err != nil {
		return err
	}
	for _, branch := range resp.Data {
		fmt.Println(branch.Name)
	}
	if resp.Next == "" {
		break
	}
	opts.PageToken = resp.Next
}
```

Each also has an `…All` iterator that follows the pages for you:

```go
for branch, err := range client.Git.ListBranchesAll(ctx, repoID, nil) {
	if err != nil {
		return err
	}
	fmt.Println(branch.Name)
}
```

Breaking out of the loop stops the requests. To resume a listing later, save a response's `Next` and pass it back as `PageToken`.

## Errors

API failures are `*entire.HTTPError` values, and their message includes the status and the API's explanation:

```go
_, err := client.Changes.Get(ctx, repoID, 42, nil)
var apiErr *entire.HTTPError
switch {
case err == nil:
case entire.IsNotFound(err):
	// no such Change
case errors.As(err, &apiErr):
	return fmt.Errorf("%w (request ID %s)", err, apiErr.RequestID)
default:
	return err // a network or context error
}
```

`IsConflict`, `IsRateLimited`, and `IsTemporary` cover the other common cases. `HTTPError` also carries the API's error `Code`, which `HasCode` and constants such as `entire.CodeWrongCell` test, and any field-level `Errors`. Include the request ID when you contact support. The transcript methods fail with `entire.ErrTranscriptUnavailable` when Entire can't read a transcript.

## Live events

`Changes.WatchEvents` follows a Change's events as they happen, and `Git.WatchRefs` a repository's branches and tags:

```go
watch := &entire.WatchOptions{LastEventID: lastID} // an empty ID starts from now
for event, err := range client.Changes.WatchEvents(ctx, repoID, changeNumber, watch) {
	if err != nil {
		return err // the stream couldn't be opened, such as for a missing Change
	}
	lastID = event.ID
	fmt.Println(event.Type, event.TargetType, event.TargetID)
}
```

Watches reconnect and resume on their own until the context ends, waiting longer while reconnecting keeps failing. A failure that reconnecting can't fix, such as a missing Change or an invalid argument, ends the loop with that error. A Change watch resumes after the last event it received; a ref watch starts each connection with a snapshot of the current refs.

## Code review

A review publishes its summary and comments together, or not at all:

```go
review, err := client.Changes.StartReview(ctx, repoID, 42, nil)
if err != nil {
	return err
}
_, err = client.Changes.PublishReview(ctx, repoID, 42, review.Data.ReviewID, entire.ReviewPublishRequest{
	Summary: "One issue",
	Comments: []entire.NewReviewComment{{
		Location: entire.CommentLocation{
			Granularity:  entire.CommentGranularityLine,
			FilePath:     "auth/login.go",
			StartLine:    42,
			SelectedText: "user, _ := lookup(name)",
		},
		Body:     "This error is dropped.",
		Severity: entire.ReviewSeverityHigh,
	}},
})
```

Starting and publishing a review send an idempotency key, so a retried request isn't applied twice, and `PublishReview` gives each comment a client ID unless you set one.

## Configuration

`NewClientFromEnv` reads `ENTIRE_TOKEN`; `NewClient` takes the token as an argument. Both accept options:

```go
client, err := entire.NewClient(token,
	entire.WithRetryPolicy(entire.RetryPolicy{MaxAttempts: 4}),
)
```

| Option | Sets |
| --- | --- |
| `WithRetryPolicy` | Automatic retries, which are off by default |
| `WithTokenSource` | A function that supplies a fresh token for each request |
| `WithWorkloadIdentity` | Sign in as a service account with the platform's OIDC token (see [Workload identity](#workload-identity)) |
| `WithHTTPClient` | Your transport, proxy, or TLS settings |
| `WithDefaultRequestOptions` | Headers or query parameters for every request |
| `WithMaxResponseBytes` | The size limit for JSON responses, 32 MiB by default |
| `WithUserAgent` | The `User-Agent` header |

Retries apply only to requests that are safe to repeat: GET, HEAD, and OPTIONS requests, reads sent as POST, and writes carrying an `Idempotency-Key`. They follow network errors and 408, 429, 502, 503, and 504 responses, and honor `Retry-After`.

Clients are safe for concurrent use. The default HTTP client has no timeout, so that event streams can stay open; set deadlines on the context.

To run against a local or private deployment, set `WithBaseURL` and `WithCoreURL`, or the `ENTIRE_API_URL` and `ENTIRE_CORE_URL` environment variables, to its endpoints.

### Service account secrets

A script that should not act as you can run as a service account. A person with manage rights on the project creates a secret for the service account and gives the service account the grants it needs. Pass the secret as the token:

```sh
# Shown once when the secret is created.
export ENTIRE_TOKEN=ent_svc_us_01JABCDEFGHJKMNPQRSTVWXYZ0_01JZYXWVTSRQPNMKJHGFEDCBA9_aaaq…
```

A secret has the form `ent_svc_<jurisdiction>_<account>_<secret>_<random><check>`. The client checks it offline, including its checksum, so a typo or a truncated paste fails at `NewClient` with a "malformed service account secret" error that never shows the secret.

The client derives the Core from the secret: it exchanges the secret at the Core of the jurisdiction the secret names, such as `https://us.auth.entire.io` for a `us` secret. `WithCoreURL` or `ENTIRE_CORE_URL` overrides that Core, for local development and staging. The exchange returns a short-lived service account access token, which lives at most 10 minutes. The client renews it shortly before it expires, and after the API answers 401; a request that is safe to repeat is then sent once more. The secret itself never reaches the API. A custom `WithTokenSource` turns the exchange off.

### Workload identity

Code that runs on a platform that issues OIDC tokens, such as GitHub Actions, can sign in as a service account with no secret at all. Bind the platform to the service account (`ServiceAccounts.CreateBinding`, or `entire service-account bind`), then:

```go
client, err := entire.NewClient("", entire.WithWorkloadIdentity(entire.WorkloadIdentity{
	ServiceAccountID: "svc_01JABCDEFGHJKMNPQRSTVWXYZ0",
	// IDToken: nil detects GitHub Actions or a Cursor Cloud Agent. Pass
	// entire.BuildkiteIDToken(), entire.GoogleCloudIDToken(),
	// entire.IDTokenFromFile(path), or your own function elsewhere.
	// AuthURL: "https://eu.auth.entire.com", for a binding outside the US.
}))
```

The client gets an ID token for the audience `<auth URL>/api/authz/sts`, exchanges it at Core for a service account access token, and exchanges again shortly before that token expires or after a 401. There is no refresh token to store.

## Building a plugin

A plugin is an integration that customers install on their Entire projects. It reads their repositories, reports check runs and deployments on their commits, and comments on their Changes. The [`plugin`](plugin) package implements the plugin protocol: install receipts, the plugin's JWTs and installation tokens, the repository list and repository creation, check runs, deployments, Git credentials, and webhooks. It also picks the right regional host for each call:

```go
app, err := plugin.New(plugin.Config{
	Slug:        "acme-plugin",
	Issuer:      "https://plugin.acme.dev",
	Signer:      key, // ed25519.PrivateKey, or a KMS key
	KeyID:       "2026-10",
	Permissions: plugin.Permissions{Contents: plugin.Read, Checks: plugin.Write},
	Audience:    "https://plugin.acme.dev",
})
http.Handle("/.well-known/jwks.json", app.JWKSHandler())

// After the install callback: receipt, err := app.AcceptReceipt(ctx, r.URL.Query(), state)
inst, err := app.Installation(receipt.InstallationID, receipt.CoreURL)
_, err = inst.Checks.Upsert(ctx, repoID, sha, plugin.UpsertRequest{
	Branch: "main",
	Suite:  plugin.Suite{Key: "api", Name: "API", ExternalID: attemptID},
	Runs: []plugin.Run{{Key: "lint", Name: "lint", ExternalID: lintID,
		Status: plugin.Completed, Conclusion: plugin.Success, ExternalUpdatedAt: finishedAt}},
})
```

The [plugin example](examples/plugin/main.go) is a complete plugin server, and the [plugin-reader example](examples/plugin-reader/main.go) installs and reads repositories from your machine. Set `Config.Permissions` to every permission the plugin registered, `Trails` included: `Installation.Token` requests them, and Entire refuses a token for every repository that names fewer, with `repository_required`. `Checks` and `Deployments` need only their own permission, because each call uses a token for its repository. An installation's token carries every registered permission on every selected repository, so keep it away from code you trust less than the plugin, and hand such code a token from `Installation.RepositoryToken`, which reaches one repository. A plugin registered with `deployments:write` shows deployed resources and their links on a commit's Change with `Installation.Deployments.Upsert`; deployments never gate the merge, so keep writing check runs for that, and each project must accept the permission before it takes effect there. A plugin registered with `discussions:write` comments on Changes through `Installation.RepositoryClient` and the client's `Changes` discussion methods, and can edit only its own messages; resolving or retitling a discussion needs `contents:write`. A plugin registered with `findings:write` reports review findings on a Change's code with the client's `Changes.StartReview`, `PublishReview`, and `UpdateReviewComment`, and changes only the findings of its own reviews. A plugin registered with `repositories:create` creates repositories in the installation's project with `Installation.CreateRepository`, which returns once the repository is ready; the permission grants nothing else, so register `contents:write` too to push to it. `App.ListInstallations` lists every installation of the plugin with the plugin's own JWT, so a plugin that missed a receipt or an event catches up from it, and `Installation.Suspend` and `Resume` pause an installation from the plugin's side. `App.WebhookHandler` verifies webhook deliveries: ref events, installation events, and, for a plugin that holds a `contents` or `trails` permission, trail events (`trail.opened`, `.updated`, `.closed`, `.reopened`, and `.merged`). Webhooks are experimental until the delivery contract is final.

## Endpoints the SDK doesn't cover

`entire.Call` sends any request with the client's authentication, retries, and error handling:

```go
type stats struct {
	Open int `json:"open"`
}
req := client.NewRequest(http.MethodGet, "/repos/"+url.PathEscape(repoID)+"/changes/stats", nil)
req.Route = entire.Route{RepoID: repoID} // send it to the cell that stores the repository
resp, err := entire.Call[stats](ctx, client, req)
```

`client.Do` returns the raw response instead. Set `Route` when the endpoint reads or changes a repository or project, and `Core` for an endpoint of Core, which holds accounts, organizations, projects, and the repository directory. To send a parameter that a method doesn't take, pass `entire.WithQueryParameter` or `entire.WithHeader`. For a listing, `entire.Iterate` builds an `…All`-style iterator from your own page function.

The API's reference is its OpenAPI documents: the [Entire API](https://aws-us-east-2.api.entire.io/openapi.json), also as [interactive documentation](https://aws-us-east-2.api.entire.io/), and [Core](https://us.auth.entire.io/api/v1/openapi.json). They move to entire.com with the rest of Entire's hosts.

## More

- Until 1.0, minor versions may change the API.
- [Code of conduct](CODE_OF_CONDUCT.md) and [security policy](SECURITY.md).
- [MIT License](LICENSE).
