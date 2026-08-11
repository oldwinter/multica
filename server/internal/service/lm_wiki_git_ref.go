package service

import (
	"net/url"
	"strings"
)

func SanitizeWikiGitRef(raw, ref, defaultBranchHint string) (LMWikiGitRef, error) {
	host, repositoryPath, err := parseWikiGitURL(strings.TrimSpace(raw))
	if err != nil {
		return LMWikiGitRef{}, err
	}
	return LMWikiGitRef{
		Host:              host,
		RepositoryPath:    repositoryPath,
		Ref:               normalizeLMWikiText(ref),
		DefaultBranchHint: normalizeLMWikiText(defaultBranchHint),
	}, nil
}

func parseWikiGitURL(raw string) (string, string, error) {
	if parsed, err := url.Parse(raw); err == nil && parsed.Host != "" {
		if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "ssh" {
			return "", "", &LMWikiUnsafeSourceError{Reason: "unsupported repository scheme"}
		}
		return validateWikiRepoPath(strings.ToLower(parsed.Hostname()), parsed.Path)
	}
	if strings.Contains(raw, "://") || strings.ContainsAny(raw, " \t\r\n?#") {
		return "", "", &LMWikiUnsafeSourceError{Reason: "invalid repository reference"}
	}
	colon := strings.IndexByte(raw, ':')
	if colon <= 0 || colon == len(raw)-1 || strings.Contains(raw[colon+1:], "@") {
		return "", "", &LMWikiUnsafeSourceError{Reason: "invalid scp repository reference"}
	}
	host := raw[:colon]
	if at := strings.IndexByte(host, '@'); at >= 0 {
		host = host[at+1:]
	}
	return validateWikiRepoPath(strings.ToLower(host), raw[colon+1:])
}

func validateWikiRepoPath(host, path string) (string, string, error) {
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	parts := strings.Split(path, "/")
	if host == "" || len(parts) < 2 {
		return "", "", &LMWikiUnsafeSourceError{Reason: "repository host or path is missing"}
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", "", &LMWikiUnsafeSourceError{Reason: "repository path is unsafe"}
		}
	}
	return host, strings.Join(parts, "/"), nil
}
