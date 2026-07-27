package service

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const pluginSkillMaxBytes = 256 * 1024

var pluginSkillHTTPClient = &http.Client{Timeout: 10 * time.Second}

// ResolveGithubSkillURL converts a GitHub repo/tree URL (or an already-raw
// URL) into the raw file URL for its SKILL.md.
func ResolveGithubSkillURL(source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", fmt.Errorf("skill source is empty")
	}
	u, err := url.Parse(source)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid skill source URL: %s", source)
	}
	host := strings.ToLower(u.Host)
	if host == "raw.githubusercontent.com" {
		return source, nil
	}
	if host != "github.com" && host != "www.github.com" {
		return "", fmt.Errorf("unsupported skill source host: %s", u.Host)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid GitHub repo URL: %s", source)
	}
	owner, repo := parts[0], parts[1]
	ref := "HEAD"
	pathParts := []string{}
	if len(parts) > 2 {
		// expect /tree/{ref}/{path...}
		if parts[2] != "tree" || len(parts) < 4 {
			return "", fmt.Errorf("unsupported GitHub URL shape: %s", source)
		}
		ref = parts[3]
		pathParts = parts[4:]
	}
	rawPath := append([]string{owner, repo, ref}, pathParts...)
	return "https://raw.githubusercontent.com/" + strings.Join(rawPath, "/") + "/SKILL.md", nil
}

// FetchPluginSkill downloads the skill markdown from source. source may be a
// GitHub URL (resolved here) or any direct raw URL.
func FetchPluginSkill(source string) (string, int64, error) {
	rawURL, err := ResolveGithubSkillURL(source)
	if err != nil {
		// Not a GitHub URL — allow plain http(s) passthrough so tests and
		// non-GitHub raw hosts still work.
		u, perr := url.Parse(source)
		if perr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return "", 0, err
		}
		rawURL = source
	}
	resp, err := pluginSkillHTTPClient.Get(rawURL)
	if err != nil {
		return "", 0, fmt.Errorf("fetch skill: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("fetch skill: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, pluginSkillMaxBytes+1))
	if err != nil {
		return "", 0, fmt.Errorf("read skill: %w", err)
	}
	if len(body) > pluginSkillMaxBytes {
		return "", 0, fmt.Errorf("skill exceeds %d bytes", pluginSkillMaxBytes)
	}
	content := strings.TrimSpace(string(body))
	if content == "" {
		return "", 0, fmt.Errorf("skill content is empty")
	}
	return content, common.GetTimestamp(), nil
}
