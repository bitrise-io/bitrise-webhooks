// Package entire is a client for Entire: repositories and Git, Trails,
// Changes, and code review, the agent sessions and checkpoints behind
// commits, search, and the organizations and projects around them.
//
// # Clients and services
//
// Create a Client with NewClient, or NewClientFromEnv, and an access token,
// such as the output of `entire auth token`. Its services group the API by
// resource, such as Client.Changes, Client.Git, and Client.Checkpoints, and
// every request goes where its data lives: repository requests to the cell
// that stores the repository, and account, organization, and project requests
// to Core, the control plane. Client.ResolveRepoID turns a repository path,
// such as "gh/acme/widget", into the ID that the repository methods take.
//
// Client.Agents, Client.Environments, Client.Runs, and Client.Runner cover
// project agents, where they run, their runs, and self-hosted runners. These
// endpoints are not yet part of Entire's published API and may change
// without a major SDK release.
//
// Methods take a context, then the path arguments, then an optional Options
// struct, which may be nil, then the request body, and finally any
// RequestOption values.
//
// # Results
//
// JSON methods return a Response, whose Data is the decoded result and whose
// body the SDK has read and closed; WithResponseBody also copies the raw
// bytes. OpenFile and RawTranscript return a reader that the caller must
// close. Watch methods, such as Changes.WatchEvents, follow live events and
// reconnect on their own. Failures are *HTTPError values; IsNotFound and the
// other helpers classify them.
//
// # Pagination
//
// A list method returns one page, and Response.Next continues the listing:
// pass it back as the PageToken of the method's ListOptions. Every list
// method also has an All iterator that follows the pages, and Iterate builds
// one for endpoints the SDK does not cover.
//
// # Types, retries, and concurrency
//
// Types are plain structs that mirror the API, with pointers only where nil
// means something. Times are Timestamp values, which are zero when the API
// sent none. Update requests use pointers, made with Ptr, for the fields to
// change, and Nullable where a field can be cleared. Enums that callers filter
// or set on are typed string constants, and values the SDK does not know
// still decode.
//
// The SDK retries nothing unless a RetryPolicy is set. Clients are immutable
// and safe for concurrent use.
//
// # Endpoints the SDK does not cover
//
// NewRequest, Do, and Call send any request with the client's
// authentication, routing, retries, and error handling. Set Request.Route
// when the endpoint addresses a repository or project, and Request.Core when
// it belongs to Core.
package entire
