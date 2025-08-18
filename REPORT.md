# GitHub Webhook Processing Review Report

## Resources

[GitHub API docs for webhook API](https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request)

## Overview

This report analyzes the GitHub webhook processing logic in the bitrise-webhooks service, focusing on potential bugs, questionable patterns, and issues that don't align with typical CI system expectations.

## Critical Issues Found

### 2. Missing Repository URL in Issue Comments
**File**: `service/hook/github/github.go:454-456`

**Issue**: For issue comment events, only `HeadRepositoryURL` and `PullRequestRepositoryURL` are populated, but `BaseRepositoryURL` is missing.

```go
HeadRepositoryURL:                eventModel.Repo.getRepositoryURL(),
PullRequestRepositoryURL:         eventModel.Repo.getRepositoryURL(),
// BaseRepositoryURL is missing
```

**Impact**: Could break build triggers that depend on the base repository URL for checkout operations.

### 3. Incomplete PR Data in Issue Comments
**File**: `service/hook/github/github.go:420`

**Issue**: The code explicitly acknowledges that mergeability checks can't be performed for issue comments due to insufficient payload data.

```go
// NOTE: we cannot do the other PR checks (see transformPullRequestEvent mergeability conditions) because the payload doesn't have enough data
```

**Problem**: This creates inconsistent behavior between PR events and PR comment events. Comments on unmergeable PRs will still trigger builds, potentially wasting resources.

## Questionable Design Patterns

### 4. Complex Skip Logic for PR Edits
**File**: `service/hook/github/github.go:268-278`

**Issue**: The PR edit skip logic only triggers builds if the previous title/body contained skip patterns, not the current ones.

```go
if !hookCommon.IsSkipBuildByCommitMessage(pullRequest.Changes.Title.From) && !hookCommon.IsSkipBuildByCommitMessage(pullRequest.Changes.Body.From) {
    return hookCommon.TransformResultModel{
        Error:      errors.New("pull Request edit doesn't require a build: only title and/or description was changed, and previous one was not skipped"),
        ShouldSkip: true,
    }
}
```

**Problem**: This creates confusing behavior. Users would expect current skip patterns to matter, not historical ones.

### 5. Inconsistent Draft PR Environment Variable
**File**: `service/hook/github/github.go:313-319`

**Issue**: The `GITHUB_PR_IS_DRAFT` environment variable is only set when the PR is a draft, not always present.

```go
if pullRequest.PullRequestInfo.Draft {
    buildEnvs = append(buildEnvs, bitriseapi.EnvironmentItem{
        Name:     "GITHUB_PR_IS_DRAFT",
        Value:    strconv.FormatBool(pullRequest.PullRequestInfo.Draft),
        IsExpand: false,
    })
}
```

**Better approach**: Always set the variable with true/false value for consistency in build scripts.

### 6. Weak Label Action Validation
**File**: `service/hook/github/github.go:285-290`

**Issue**: The "labeled" action is rejected if `Mergeable` is nil, but doesn't verify the PR is actually open.

```go
if pullRequest.Action == "labeled" && pullRequest.PullRequestInfo.Mergeable == nil {
    return hookCommon.TransformResultModel{
        Error:      errors.New("pull Request label added to PR that is not open yet"),
        ShouldSkip: true,
    }
}
```

**Problem**: Could allow label builds on closed PRs if mergeable happens to be calculated.

## Security and Reliability Concerns

### 7. Repository URL Selection Logic
**File**: `service/hook/github/github.go:581-586`

**Issue**: Private repositories automatically use SSH URLs while public repositories use HTTPS URLs.

```go
func (repoInfoModel RepoInfoModel) getRepositoryURL() string {
    if repoInfoModel.Private {
        return repoInfoModel.SSHURL
    }
    return repoInfoModel.CloneURL
}
```

**Risk**: This assumes SSH keys are always available for private repos, which could cause checkout failures if SSH keys aren't properly configured.

### 8. Missing Commit Hash Validation

**Issue**: The code only checks for empty commit hashes but doesn't validate the SHA format.

**Risk**: Malformed commit hashes could cause downstream build system issues.

### 9. No Force Push Detection

**Issue**: Force pushes are processed like normal pushes without special handling.

**Problem**: CI systems often need to handle force pushes differently (e.g., cancel running builds, special security checks).

### 10. Missing Fork PR Handling

**Issue**: No explicit detection or handling of PRs from forks versus same repository.

**Security implications**: Fork PRs often require different security policies and access patterns.

## Logic Inconsistencies

### 12. Comment Content Ignored
**File**: `service/hook/github/github.go:425-428`

**Issue**: For issue comment events, the commit message uses the PR title/body instead of the actual comment that triggered the build.

```go
commitMsg := issue.Title
if issue.Body != "" {
    commitMsg = fmt.Sprintf("%s\n\n%s", commitMsg, issue.Body)
}
```

**Problem**: The actual comment content that triggered the build (`eventModel.Comment.Body`) is stored separately but not used in the commit message, making it harder to understand what triggered the build.

## Performance Issues

### 14. Missing Input Validation

**Issue**: No validation of payload size, webhook frequency, or malformed data beyond basic JSON parsing.

**Risk**: Service could be overwhelmed by malicious or broken webhook sources.
