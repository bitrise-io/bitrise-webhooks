package plugin

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
)

// GitEnv returns environment variables that let ordinary Git clone and
// fetch repo.CloneURL over HTTPS, with the installation's token, which needs
// contents:read registered.
// Append them to the command's environment:
//
//	cmd := exec.CommandContext(ctx, "git", "clone", repo.CloneURL, dir)
//	cmd.Env = append(os.Environ(), env...)
//
// The token travels in an http.extraHeader scoped to the repository's host,
// set through GIT_CONFIG_COUNT, so it never appears in the URL, the command
// line, or the repository's .git/config. The token carries the
// installation's full authority (see Token), so run Git only in an
// environment you trust as much as the plugin. The variables replace any
// GIT_CONFIG_COUNT already in the environment. The token lasts about 45
// minutes; call GitEnv again for later fetches.
func (i *Installation) GitEnv(ctx context.Context, repo Repository) ([]string, error) {
	if repo.CloneURL == "" {
		return nil, errors.New("plugin: repository has no clone URL; list the repositories again")
	}
	u, err := i.app.checkURL(repo.CloneURL, "")
	if err != nil {
		return nil, err
	}
	if u.Scheme == "entire" {
		return nil, fmt.Errorf("plugin: %s is not an HTTPS clone URL", u.Redacted())
	}
	token, err := i.Token(ctx)
	if err != nil {
		return nil, err
	}
	return []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=http." + u.Scheme + "://" + u.Host + "/.extraHeader",
		"GIT_CONFIG_VALUE_0=Authorization: " + basicAuthorization(token),
	}, nil
}

// GitAuthorization returns the value of an Authorization header that lets a
// Git smart-HTTP request to repo.CloneURL, or to the node it redirects to,
// act for the installation, such as "Basic eC1hY2Nlc3M...". It suits code
// that speaks the protocol itself, such as a proxy, where GitEnv suits code
// that runs Git:
//
//	auth, err := inst.GitAuthorization(ctx, repo, plugin.Permissions{Contents: plugin.Read})
//	req.Header.Set("Authorization", auth)
//
// The token is RepositoryToken's, narrowed to repo and the permissions
// requested, and is cached and renewed as RepositoryToken's are, so call
// GitAuthorization for every request rather than keeping its result. Send
// the header only to repo.CloneURL's host and to the hosts it redirects to
// that HostPolicy approves: it carries a credential.
func (i *Installation) GitAuthorization(ctx context.Context, repo Repository, permissions Permissions) (string, error) {
	if repo.CloneURL == "" {
		return "", errors.New("plugin: repository has no clone URL; list the repositories again")
	}
	u, err := i.app.checkURL(repo.CloneURL, "")
	if err != nil {
		return "", err
	}
	if u.Scheme == "entire" {
		return "", fmt.Errorf("plugin: %s is not an HTTPS clone URL", u.Redacted())
	}
	token, err := i.RepositoryToken(ctx, repo.ID, permissions)
	if err != nil {
		return "", err
	}
	return basicAuthorization(token), nil
}

// basicAuthorization is the HTTP Basic credential Git sends for token.
func basicAuthorization(token *Token) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(token.Username+":"+token.Value))
}

// RemoteHelperEnv returns environment variables that let Git clone and
// fetch repo.EntireCloneURL through git-remote-entire, with the
// installation's token in ENTIRE_TOKEN. Use it only where git-remote-entire is
// installed; GitEnv needs no Entire software.
func (i *Installation) RemoteHelperEnv(ctx context.Context, repo Repository) ([]string, error) {
	if repo.EntireCloneURL == "" {
		return nil, errors.New("plugin: repository has no entire clone URL; list the repositories again")
	}
	if _, err := i.app.checkURL(repo.EntireCloneURL, "entire"); err != nil {
		return nil, err
	}
	token, err := i.Token(ctx)
	if err != nil {
		return nil, err
	}
	return []string{"GIT_TERMINAL_PROMPT=0", "ENTIRE_TOKEN=" + token.Value}, nil
}
