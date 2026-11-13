// Package plugin builds Entire plugins: integrations that a customer
// installs on an Entire project to read its repositories, report check runs
// and deployments on its commits, comment on its trails and report review
// findings on them, and create repositories in it.
//
// A plugin is a server. An Entire admin registers it once, under Admin →
// Plugins, with its slug, issuer, permissions, and Ed25519 public keys:
// either the URL of a JWKS the plugin serves, or the keys themselves as PEM,
// which PublicKeyPEM prints. ParsePrivateKey reads a key that openssl made.
// From then on, the SDK handles the protocol:
//
//	app, err := plugin.New(plugin.Config{
//		Slug:        "acme-plugin",
//		Issuer:      "https://plugin.acme.dev",
//		Signer:      key, // an ed25519.PrivateKey, or a KMS key
//		KeyID:       "2026-10",
//		Permissions: plugin.Permissions{Contents: plugin.Read, Checks: plugin.Write},
//		Audience:    "https://plugin.acme.dev",
//	})
//
//	// Serve the JWKS registered with Entire. A plugin registered with
//	// PEM keys serves none, and may leave KeyID empty.
//	http.Handle("/.well-known/jwks.json", app.JWKSHandler())
//
// # Installing
//
// The plugin starts an install by sending the customer's browser to
// App.InstallURL with a state from NewState. Entire sends the customer back
// to the plugin's callback with a signed receipt, which App.AcceptReceipt
// verifies: it must come from one of Config.Cores. Store the receipt's
// InstallationID. Installations are global, so any Core serves them; the
// receipt's CoreURL, the Core of the project's jurisdiction, is the best one
// to call, and Config.CoreURL pins one for every installation. A receipt
// whose Action is ActionUpdate names an installation the plugin may hold
// already, after the customer changed it.
//
// A plugin can miss a receipt, such as when the customer closes the browser
// before the callback, or when the plugin was down. App.ListInstallations
// lists every installation of the plugin, with the plugin's own JWT, so it
// is how a plugin catches up: run it at startup or on a schedule, and
// compare it with the installations you store:
//
//	for summary, err := range app.ListInstallationsAll(ctx, plugin.DefaultCores[0], nil) {
//		if err != nil {
//			return err
//		}
//		if summary.LifecycleState == plugin.LifecycleActive && !known(summary.ID) {
//			inst, err := app.Installation(summary.ID, plugin.DefaultCores[0])
//			// ...
//		}
//	}
//
// Installation.Suspend pauses an installation from the plugin's side, and
// Installation.Resume ends that pause. The customer can suspend too, and
// only the side that suspended can resume. While suspended, calls for the
// installation fail with 403 and code CodeInstallationSuspended.
//
// # Acting for an installation
//
// App.Installation returns an Installation. It mints and caches the
// installation's token, and finds the host that serves each repository, so
// the plugin never handles a token or chooses a host:
//
//	inst, err := app.Installation(receipt.InstallationID, receipt.CoreURL)
//	repos, err := inst.Repositories(ctx)
//	_, err = inst.Checks.Upsert(ctx, repo.ID, sha, plugin.UpsertRequest{
//		Branch: "main",
//		Suite:  plugin.Suite{Key: "api", Name: "API", ExternalID: attemptID},
//		Runs: []plugin.Run{{Key: "lint", Name: "lint", ExternalID: lintID,
//			Status: plugin.InProgress, ExternalUpdatedAt: startedAt}},
//	})
//
// An installation's token carries every registered permission on every
// repository the customer selected. Keep it, and the Git credentials made
// from it, away from code you trust less than the plugin.
// Installation.RepositoryToken narrows a token to one repository and fewer
// permissions, and Installation.RepositoryClient returns an ordinary SDK
// client that uses one:
//
//	client, repoID, err := inst.RepositoryClient(ctx, repo.ID, plugin.Permissions{Contents: plugin.Read})
//	readme, err := client.Git.ReadFile(ctx, repoID, "HEAD", "README.md")
//
// A plugin registered with Discussions: Write comments on Changes through
// the same client. It can edit and delete only its installation's own
// messages, from any of the installation's tokens. Resolving or retitling
// a discussion needs Contents: Write, and starting a code review needs
// Findings: Write, below:
//
//	client, repoID, err := inst.RepositoryClient(ctx, repo.ID,
//		plugin.Permissions{Contents: plugin.Read, Discussions: plugin.Write})
//	created, err := client.Changes.CreateDiscussion(ctx, repoID, changeNumber,
//		entire.DiscussionCreateRequest{Title: "Deploy previews", Body: summary})
//	_, err = client.Changes.UpdateMessage(ctx, repoID, changeNumber,
//		created.Data.ID, created.Data.Messages[0].ID, entire.MessageRequest{Body: updated})
//
// A plugin registered with Findings: Write reports review findings on a
// Change's code through the same client, as a code review it owns, and can
// change only the findings of its own reviews. Like Discussions, it reads
// nothing: Installation.RepositoryClient also needs Contents: Read. Start a
// review for the commit you analyzed, publish every finding in one call,
// and later update a finding's status as pushes fix it. Findings never gate
// the merge; report a check run for that:
//
//	client, repoID, err := inst.RepositoryClient(ctx, repo.ID,
//		plugin.Permissions{Contents: plugin.Read, Findings: plugin.Write})
//	review, err := client.Changes.StartReview(ctx, repoID, changeNumber,
//		&entire.ReviewStartOptions{HeadSHA: sha})
//	published, err := client.Changes.PublishReview(ctx, repoID, changeNumber, review.Data.ReviewID,
//		entire.ReviewPublishRequest{Summary: summary, Comments: []entire.NewReviewComment{{
//			ClientID: "sql-injection:api/users.go:42",
//			Location: entire.CommentLocation{Granularity: entire.CommentGranularityLine,
//				FilePath: "api/users.go", StartLine: 42, SelectedText: line},
//			Body: "User input reaches this query unescaped.", Severity: entire.ReviewSeverityHigh,
//		}}})
//	_, err = client.Changes.UpdateReviewComment(ctx, repoID, changeNumber, review.Data.ReviewID,
//		commentID, entire.ReviewCommentUpdate{Status: entire.ReviewCommentStatusResolved,
//			StatusReason: entire.Value("Fixed in " + fixSHA)})
//
// A plugin registered with Deployments: Write shows the resources it
// deployed, with their links, on a commit's Change. Deployments never gate
// the merge, so a plugin that gates merges keeps writing check runs too.
// Keep each resource's key stable across updates and commits, and report
// Inactive when the resource is removed:
//
//	_, err = inst.Deployments.Upsert(ctx, repo.ID, sha, plugin.DeploymentsRequest{
//		Branch: "feature-x",
//		Deployments: []plugin.Deployment{{Key: "preview-site", Name: "Preview site",
//			Status: plugin.Completed, Conclusion: plugin.Success,
//			EnvironmentURL: "https://preview.example.com", ExternalUpdatedAt: deployedAt}},
//	})
//
// Installation.GitEnv clones a repository with ordinary Git, and
// Installation.RemoteHelperEnv with git-remote-entire.
//
// # Creating repositories
//
// A plugin registered with Repositories: Create creates repositories in the
// installation's project. The permission creates and nothing more: register
// Contents: Write as well to push to the repository the plugin created.
// Installation.CreateRepository returns the repository once it is ready,
// and adds it to the installation, even one limited to selected
// repositories:
//
//	repo, err := inst.CreateRepository(ctx, "widget", nil)
//	if errors.Is(err, plugin.ErrRepositoryExists) {
//		// The project has a repository named widget already.
//	}
//	auth, err := inst.GitAuthorization(ctx, repo, plugin.Permissions{Contents: plugin.Write})
//
// Entire limits creates per installation, by default to 100 in any 24
// hours, and answers more with ErrCreateLimit.
//
// # Events
//
// App.WebhookHandler serves the plugin's webhook URL: it verifies each
// delivery's signature and body digest, skips deliveries already handled,
// and passes the rest to the plugin. Entire delivers ref events, to a plugin
// that holds contents:read; installation events; and trail events, to a
// plugin that holds contents or trails permission. Decode an
// event's payload with Event.RefChange, Event.InstallationChange, or
// Event.TrailChange. Webhooks are experimental until the delivery contract is
// final.
//
// # Hosts
//
// Entire runs in several regions. The SDK takes every host from Entire, from
// the repository list or a delivery, and sends credentials only to Cores in
// Config.Cores and to https hosts under entire.com (or entire.io, until
// Entire's move to entire.com ends). Config.Cores and
// Config.HostPolicy replace those rules for a non-production deployment or a
// local test server.
//
// With Config.Cores empty, the plugin trusts every host named
// https://{jurisdiction}.auth.entire.com (or .auth.entire.io) as a Core, so
// a Core that Entire opens after this SDK's release needs no upgrade.
//
// # Errors
//
// Calls return *entire.HTTPError for unsuccessful responses. Branch on its
// StatusCode and Code, such as entire.CodeUnsupportedPermission, and include
// its RequestID when reporting a problem to Entire. Calls are retried on
// 429, 502, 503 (including no_primary), and 504 with identical bodies, and
// check-run and deployment calls follow a repository that moved (409
// wrong_cell). A 403 is
// never retried: a token mint answers 403 both when it refuses and when
// mints come too fast, which the token cache avoids.
//
// # Not yet final
//
// Deployments are on Entire's v0 API, which may change.
package plugin
