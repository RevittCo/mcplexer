package gateway

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	reRepoQualifier = regexp.MustCompile(`(?i)\brepo:([a-z0-9_.-]+)/([a-z0-9_.-]+)\b`)
	reOrgQualifier  = regexp.MustCompile(`(?i)\borg:([a-z0-9_.-]+)\b`)
)

type githubScopePolicy struct {
	allowedOrgs  map[string]struct{}
	allowedRepos map[string]struct{}
}

func newGitHubScopePolicy(rawOrgs, rawRepos json.RawMessage) (*githubScopePolicy, error) {
	orgs, err := parseStringArray(rawOrgs)
	if err != nil {
		return nil, fmt.Errorf("allowed_orgs: %w", err)
	}
	repos, err := parseStringArray(rawRepos)
	if err != nil {
		return nil, fmt.Errorf("allowed_repos: %w", err)
	}

	p := &githubScopePolicy{
		allowedOrgs:  make(map[string]struct{}, len(orgs)),
		allowedRepos: make(map[string]struct{}, len(repos)),
	}
	for _, org := range orgs {
		org = strings.ToLower(strings.TrimSpace(org))
		if org == "" {
			continue
		}
		p.allowedOrgs[org] = struct{}{}
	}
	for _, repo := range repos {
		repo = normalizeRepo(repo)
		if repo == "" {
			continue
		}
		p.allowedRepos[repo] = struct{}{}
	}
	return p, nil
}

func (p *githubScopePolicy) enabled() bool {
	return len(p.allowedOrgs) > 0 || len(p.allowedRepos) > 0
}

func (p *githubScopePolicy) Enforce(args json.RawMessage) error {
	if !p.enabled() {
		return nil
	}

	targets := extractGitHubTargets(args)
	if len(targets.repos) == 0 && len(targets.orgs) == 0 {
		return nil
	}

	for repo := range targets.repos {
		owner := repoOwner(repo)
		if _, ok := p.allowedRepos[repo]; ok {
			continue
		}
		if _, ok := p.allowedOrgs[owner]; ok {
			continue
		}
		return fmt.Errorf("github repo %q is not allowed by route policy", repo)
	}

	for org := range targets.orgs {
		if targets.hasRepoForOrg(org) {
			continue
		}
		if _, ok := p.allowedOrgs[org]; ok {
			continue
		}
		if len(p.allowedOrgs) == 0 && len(p.allowedRepos) > 0 {
			return fmt.Errorf("github org %q is not allowed by route policy", org)
		}
		if len(p.allowedOrgs) > 0 {
			return fmt.Errorf("github org %q is not allowed by route policy", org)
		}
	}

	return nil
}

type githubTargets struct {
	orgs  map[string]struct{}
	repos map[string]struct{}
}

func newGitHubTargets() githubTargets {
	return githubTargets{
		orgs:  map[string]struct{}{},
		repos: map[string]struct{}{},
	}
}

func (t githubTargets) addOrg(org string) {
	org = strings.ToLower(strings.TrimSpace(org))
	if org == "" {
		return
	}
	t.orgs[org] = struct{}{}
}

func (t githubTargets) addRepo(repo string) {
	repo = normalizeRepo(repo)
	if repo == "" {
		return
	}
	t.repos[repo] = struct{}{}
	t.addOrg(repoOwner(repo))
}

func (t githubTargets) hasRepoForOrg(org string) bool {
	for repo := range t.repos {
		if repoOwner(repo) == org {
			return true
		}
	}
	return false
}

func extractGitHubTargets(args json.RawMessage) githubTargets {
	targets := newGitHubTargets()
	if len(args) == 0 {
		return targets
	}

	var data any
	if err := json.Unmarshal(args, &data); err != nil {
		return targets
	}
	walkGitHubArgs(data, &targets)
	return targets
}

func walkGitHubArgs(v any, targets *githubTargets) {
	switch val := v.(type) {
	case map[string]any:
		extractFromMap(val, targets)
		for _, child := range val {
			walkGitHubArgs(child, targets)
		}
	case []any:
		for _, item := range val {
			walkGitHubArgs(item, targets)
		}
	case string:
		extractFromString(val, targets)
	}
}

func extractFromMap(m map[string]any, targets *githubTargets) {
	owner := asString(m["owner"])
	repo := asString(m["repo"])
	if owner != "" && repo != "" {
		targets.addRepo(owner + "/" + repo)
	}

	if org := asString(m["org"]); org != "" {
		targets.addOrg(org)
	}
	if org := asString(m["organization"]); org != "" {
		targets.addOrg(org)
	}

	if fullName := asString(m["full_name"]); fullName != "" {
		targets.addRepo(fullName)
	}
	if repoField := asString(m["repository"]); repoField != "" {
		targets.addRepo(repoField)
	}
	if repoField := asString(m["repository_name"]); repoField != "" {
		targets.addRepo(repoField)
	}

	if repoObj, ok := m["repository"].(map[string]any); ok {
		repoOwner := asString(repoObj["owner"])
		repoName := asString(repoObj["name"])
		if repoOwner != "" && repoName != "" {
			targets.addRepo(repoOwner + "/" + repoName)
		}
	}

	for _, key := range []string{"url", "html_url", "repository_url", "clone_url"} {
		if raw := asString(m[key]); raw != "" {
			extractFromGitHubURL(raw, targets)
		}
	}

	for _, key := range []string{"query", "q", "search", "text"} {
		if raw := asString(m[key]); raw != "" {
			extractFromQueryString(raw, targets)
		}
	}
}

func extractFromString(s string, targets *githubTargets) {
	extractFromGitHubURL(s, targets)
	extractFromQueryString(s, targets)
	if strings.Count(strings.TrimSpace(s), "/") == 1 {
		targets.addRepo(s)
	}
}

func extractFromQueryString(s string, targets *githubTargets) {
	for _, m := range reRepoQualifier.FindAllStringSubmatch(s, -1) {
		if len(m) == 3 {
			targets.addRepo(m[1] + "/" + m[2])
		}
	}
	for _, m := range reOrgQualifier.FindAllStringSubmatch(s, -1) {
		if len(m) == 2 {
			targets.addOrg(m[1])
		}
	}
}

func extractFromGitHubURL(raw string, targets *githubTargets) {
	u, err := url.Parse(raw)
	if err != nil {
		return
	}
	host := strings.ToLower(u.Host)
	if host != "github.com" && host != "www.github.com" && host != "api.github.com" {
		return
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 2 {
		if parts[0] == "repos" && len(parts) >= 3 {
			targets.addRepo(parts[1] + "/" + parts[2])
			return
		}
		targets.addRepo(parts[0] + "/" + parts[1])
	}
}

func parseStringArray(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("must be a JSON array of strings")
	}
	return out, nil
}

func repoOwner(repo string) string {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[0]
}

func normalizeRepo(repo string) string {
	repo = strings.ToLower(strings.TrimSpace(repo))
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return ""
	}
	owner := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	if owner == "" || name == "" {
		return ""
	}
	return owner + "/" + name
}

func asString(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}
