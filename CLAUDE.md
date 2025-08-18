# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

### Testing
- `bitrise run test` - Run complete test suite (Go tests, go vet, errcheck, golint)
- `go test ./...` - Run Go tests only
- `go test ./service/hook/github/...` - Run tests for specific package

### Development
- `bitrise run start` - Start development server with live reload using gin
- `go install github.com/bitrise-io/bitrise-webhooks && bitrise-webhooks -port 4000` - Run server directly

### Build Tools
- `go install` - Build the project
- `go vet ./...` - Static analysis
- `errcheck -asserts=true -blank=true $(go list ./... | grep -v vendor)` - Check for unhandled errors
- `golint ./...` - Lint Go code

## Architecture Overview

This is a webhook processing service that receives webhooks from Git providers (GitHub, GitLab, Bitbucket, etc.) and transforms them into Bitrise build triggers.

### Core Components

1. **HTTP Router** (`routes.go`) - Single main endpoint `/h/{service-id}/{app-slug}/{api-token}`
2. **Provider System** (`service/hook/`) - Pluggable providers for different webhook formats
3. **Build Triggering** (`bitriseapi/`) - Transforms webhooks to Bitrise API calls
4. **Metrics** (`metrics/`, `internal/pubsub/`) - Optional PubSub-based analytics

### Provider Pattern Architecture

Each provider implements the `Provider` interface with `TransformRequest(r *http.Request) TransformResultModel`:

#### Core Interfaces (`service/hook/common/common.go`)
- **Provider**: Main transformation interface
- **ResponseTransformer**: Optional custom response formatting
- **MetricsProvider**: Optional webhook analytics collection

#### Provider Registry (`service/hook/endpoint.go`)
All providers registered in `supportedProviders()` map linking URL paths to provider implementations.

### Git Provider Event Handling Patterns

#### GitHub Provider (`service/hook/github/`)
**Events**: Push (`X-Github-Event: push`), Pull Request (`pull_request`), Issue Comments (`issue_comment`)
**Features**:
- Validates merge refs using `mergeable` field
- Handles draft PRs with `GITHUB_PR_IS_DRAFT` env var
- Processes PR labels and label additions
- Creates merge/head branch refs: `pull/{id}/merge`, `pull/{id}/head`
- Skip logic: deleted branches, merged PRs, non-mergeable PRs

#### GitLab Provider (`service/hook/gitlab/`)
**Events**: Push Hook, Tag Push Hook, Merge Request Hook, Note Hook (comments)
**Features**:
- Always async processing (`DontWaitForTriggerResponse: true`)
- Sophisticated commit message truncation for env var size limits
- Repository URL selection based on visibility level
- Merge refs: `merge-requests/{id}/merge` and `merge-requests/{id}/head`
- Handles squashed merge scenarios with empty `checkout_sha`

#### Bitbucket v2 Provider (`service/hook/bitbucketv2/`)
**Events**: `repo:push`, `pullrequest:created/updated`, `pullrequest:comment_created/updated`
**Features**:
- Multi-SCM support (Git and Mercurial)
- Processes multiple branch/tag changes per webhook
- Repository privacy detection via HTTP HEAD requests
- Retry prevention (rejects `X-Attempt-Number` > 1)

### Common Transformation Patterns

#### Repository URL Generation
- **GitHub**: `private ? ssh_url : clone_url`
- **GitLab**: `visibility_level == 20 ? git_http_url : git_ssh_url`
- **Bitbucket**: `is_private ? ssh : https` format

#### Universal Skip CI Logic (`service/hook/common/skipci.go`)
- `[ci skip]` / `[skip ci]` / `[bitrise skip]` / `[skip bitrise]`
- Handles escaped versions: `\[ci skip\]` and `\\[ci skip\\]`

#### Pull Request Processing
1. State validation (only open/opened PRs)
2. Merge conflict detection (skip non-mergeable)
3. Draft state tracking and transitions
4. Comment processing as build triggers
5. Label management (additions/changes)

### Build Parameter Standardization

All providers transform to `bitriseapi.TriggerAPIParamsModel` with:
- Repository info: `Branch`, `Tag`, `CommitHash`, `CommitMessage`
- PR details: `PullRequestID`, `PullRequestMergeBranch`, `PullRequestHeadBranch`
- Metadata: `PullRequestAuthor`, `PullRequestLabels`, `DiffURL`
- Environment: `Environments[]`, `PushCommitPaths[]`

### Adding New Providers

1. Create folder `service/hook/newprovider/`
2. Implement provider struct with `TransformRequest()` method
3. Handle provider-specific events and data models
4. Implement skip logic for unwanted builds
5. Add comprehensive unit tests covering all event types
6. Register in `supportedProviders()` map in `service/hook/endpoint.go`

### Metrics Collection Architecture

Providers can implement `MetricsProvider` interface for analytics:
- **PushMetrics**: Commit information and file changes
- **PullRequestMetrics**: PR statistics (files, additions, deletions)
- **PullRequestCommentMetrics**: Comment events

### Key Files
- `main.go` - Entry point and server setup
- `service/hook/endpoint.go` - Central webhook processing and provider registry
- `service/hook/common/` - Shared interfaces, skip logic, and metrics
- `bitriseapi/bitriseapi.go` - Bitrise API integration
- Provider implementations: `service/hook/{github,gitlab,bitbucketv2,etc}/`

### Environment Variables
- `PORT` - Server port (default 4000)
- `RACK_ENV=production` - Enable production mode (sends actual build requests)
- `SEND_REQUEST_TO` - Override build trigger URL for testing
- `IS_USE_GIN=yes` - Use gin for development live reload

### Provider Testing Patterns
Each provider should have comprehensive tests covering:
- All supported event types and their variations
- Skip logic scenarios (deleted branches, merged PRs, etc.)
- Error cases (malformed payloads, missing headers)
- Multiple build triggers per webhook
- Edge cases specific to the provider's webhook format