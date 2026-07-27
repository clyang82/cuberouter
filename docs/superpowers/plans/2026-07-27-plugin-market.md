# Plugin Market Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Admins register plugins (remote MCP server + skill markdown fetched from GitHub); playground users invoke them with `@slug`, and the gateway runs the MCP tool-call loop server-side before streaming the final answer.

**Architecture:** New `plugins` GORM table + admin CRUD; new `pkg/mcp` JSON-RPC client; a per-request agentic loop in `controller.Playground` that drives `controller.Relay` against recorder-backed sub-contexts (channel-test.go pattern), injecting skill content into the system prompt and MCP tools into `request.Tools`; frontend gets an admin plugins page and `@` autocomplete in the playground input.

**Tech Stack:** Go 1.22 (Gin, GORM, testify), React 19 + TypeScript + TanStack Router + Base UI + Tailwind, Bun.

## Global Constraints

- All JSON marshal/unmarshal in Go MUST use `common.Marshal` / `common.Unmarshal` / `common.UnmarshalJsonStr` from `common/json.go`. Never import `encoding/json` for calls in business code (type references like `json.RawMessage` are fine).
- DB code MUST work on SQLite, MySQL >= 5.7.8, PostgreSQL >= 9.6. No JSON column types for the new table (use `type:text`), no dialect-specific SQL, no `gorm:"default:true"` boolean tags.
- New Go tests MUST use `github.com/stretchr/testify/require` for setup/fatal assertions and `github.com/stretchr/testify/assert` for value checks. Deterministic table tests; use `httptest.Server` for any HTTP dependency — never the real GitHub or a real MCP server.
- Frontend: Bun (`bun install`, `bun run typecheck`, `bun run lint`, `bun run i18n:sync`). All user-facing text via `t('English key')`. Every new source file starts with the AGPL copyright header — run `bun run copyright` if unsure.
- Do not modify protected project identity (new-api / QuantumNous references).
- Slug rules: `^[a-z0-9][a-z0-9-]{1,63}$`, unique, immutable after creation.
- Limits: max 3 plugins per message, max 10 tool rounds, skill fetch 10s timeout + 256 KiB cap, MCP timeouts 5s connect / 10s ListTools / 30s CallTool, MCP response cap 1 MiB, tool list cache TTL 60s.

---

### Task 1: Plugin model + migration

**Files:**
- Create: `model/plugin.go`
- Modify: `model/main.go` (both `migrateDB()` ~L271-305 and `migrateDBFast()` ~L331-366)
- Test: `model/plugin_test.go`

**Interfaces:**
- Consumes: `model.DB`, `common.GetTimestamp()`, existing test helper `model.SetupTestDB(t)` if present (verify with `grep -rn "func SetupTestDB" model/`; if absent, use the sqlite in-memory pattern from an existing `model/*_test.go`).
- Produces:
  - `type Plugin struct { Id int; Name string; Slug string; Description string; Enabled bool; McpUrl string; SkillSource string; SkillContent string; SkillFetchedAt int64; CreatedTime int64; UpdatedTime int64 }`
  - `var PluginSlugRegex = regexp.MustCompile("^[a-z0-9][a-z0-9-]{1,63}$")`
  - `func ValidatePluginSlug(slug string) bool`
  - `func (p *Plugin) Insert() error`
  - `func (p *Plugin) Update() error`
  - `func DeletePluginByID(id int) error`
  - `func GetAllPlugins() ([]*Plugin, error)`
  - `func GetPluginByID(id int) (*Plugin, error)`
  - `func IsPluginSlugDuplicated(id int, slug string) (bool, error)`

- [ ] **Step 1: Write the failing test**

`model/plugin_test.go` (follow the DB setup pattern of an existing model test — check `ls model/*_test.go` and copy its fixture):

```go
package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePluginSlug(t *testing.T) {
	cases := []struct {
		slug  string
		valid bool
	}{
		{"web-search", true},
		{"a1", true},
		{"ab", true},
		{"a", false},               // too short
		{"-abc", false},            // leading dash
		{"Web", false},             // uppercase
		{"web_search", false},      // underscore
		{"web search", false},      // space
		{string(make([]byte, 65)), false}, // too long (also invalid chars)
	}
	for _, tc := range cases {
		assert.Equal(t, tc.valid, ValidatePluginSlug(tc.slug), "slug=%q", tc.slug)
	}
}

func TestPluginCRUD(t *testing.T) {
	// Use the same sqlite in-memory setup as existing model tests.
	setupTestDBForPlugin(t) // see note below

	p := &Plugin{
		Name:        "Web Search",
		Slug:        "web-search",
		Description: "search the web",
		Enabled:     true,
		McpUrl:      "https://mcp.example.com/mcp",
		SkillSource: "https://github.com/acme/web-search-skill",
	}
	require.NoError(t, p.Insert())
	require.NotZero(t, p.Id)
	require.NotZero(t, p.CreatedTime)

	got, err := GetPluginByID(p.Id)
	require.NoError(t, err)
	assert.Equal(t, "web-search", got.Slug)
	assert.True(t, got.Enabled)

	dup, err := IsPluginSlugDuplicated(0, "web-search")
	require.NoError(t, err)
	assert.True(t, dup)
	dup, err = IsPluginSlugDuplicated(p.Id, "web-search")
	require.NoError(t, err)
	assert.False(t, dup)

	got.Enabled = false
	require.NoError(t, got.Update())
	all, err := GetAllPlugins()
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.False(t, all[0].Enabled)

	require.NoError(t, DeletePluginByID(p.Id))
	all, err = GetAllPlugins()
	require.NoError(t, err)
	assert.Empty(t, all)
}
```

NOTE for the implementer: before writing this file, run `ls model/*_test.go` and open one to copy its DB fixture. If the repo's model tests use a helper (e.g. resetting `model.DB` to a fresh `gorm.Open(sqlite.Open("file::memory:?cache=shared"))` + `DB.AutoMigrate(&Plugin{})`), define `setupTestDBForPlugin(t)` in this test file following that exact pattern. Do not invent a new global fixture.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./model/ -run 'TestValidatePluginSlug|TestPluginCRUD' -v`
Expected: FAIL — `undefined: ValidatePluginSlug`, `undefined: Plugin`

- [ ] **Step 3: Implement `model/plugin.go`**

```go
package model

import (
	"regexp"

	"github.com/QuantumNous/new-api/common"
)

var PluginSlugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)

type Plugin struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	Name           string `json:"name" gorm:"size:128;not null"`
	Slug           string `json:"slug" gorm:"size:64;not null;uniqueIndex"`
	Description    string `json:"description" gorm:"size:512"`
	Enabled        bool   `json:"enabled"`
	McpUrl         string `json:"mcp_url" gorm:"size:1024"`
	SkillSource    string `json:"skill_source" gorm:"size:1024"`
	SkillContent   string `json:"skill_content" gorm:"type:text"`
	SkillFetchedAt int64  `json:"skill_fetched_at" gorm:"bigint"`
	CreatedTime    int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime    int64  `json:"updated_time" gorm:"bigint"`
}

func ValidatePluginSlug(slug string) bool {
	return PluginSlugRegex.MatchString(slug)
}

func (p *Plugin) Insert() error {
	now := common.GetTimestamp()
	p.CreatedTime = now
	p.UpdatedTime = now
	return DB.Create(p).Error
}

func (p *Plugin) Update() error {
	p.UpdatedTime = common.GetTimestamp()
	return DB.Save(p).Error
}

func DeletePluginByID(id int) error {
	return DB.Delete(&Plugin{}, id).Error
}

func GetAllPlugins() ([]*Plugin, error) {
	var plugins []*Plugin
	if err := DB.Model(&Plugin{}).Order("updated_time DESC").Find(&plugins).Error; err != nil {
		return nil, err
	}
	return plugins, nil
}

func GetPluginByID(id int) (*Plugin, error) {
	var p Plugin
	if err := DB.First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func IsPluginSlugDuplicated(id int, slug string) (bool, error) {
	if slug == "" {
		return false, nil
	}
	var cnt int64
	err := DB.Model(&Plugin{}).Where("slug = ? AND id <> ?", slug, id).Count(&cnt).Error
	return cnt > 0, err
}
```

Register the migration in `model/main.go`:
- In `migrateDB()` add `&Plugin{},` to the `DB.AutoMigrate(...)` list (next to `&PrefillGroup{}`).
- In `migrateDBFast()` add `{&Plugin{}, "Plugin"},` to the `migrations` slice.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./model/ -run 'TestValidatePluginSlug|TestPluginCRUD' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add model/plugin.go model/plugin_test.go model/main.go
git commit -m "feat(plugin): add plugin model and migration"
```

---

### Task 2: GitHub skill fetcher

**Files:**
- Create: `service/plugin_skill.go`
- Test: `service/plugin_skill_test.go`

**Interfaces:**
- Consumes: `common.GetTimestamp()`.
- Produces:
  - `func ResolveGithubSkillURL(source string) (rawURL string, err error)` — pure function.
  - `func FetchPluginSkill(source string) (content string, fetchedAt int64, err error)` — HTTP fetch, 10s timeout, 256 KiB cap, requires HTTP 200 and non-empty body.

- [ ] **Step 1: Write the failing test**

`service/plugin_skill_test.go`:

```go
package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveGithubSkillURL(t *testing.T) {
	cases := []struct {
		source  string
		want    string
		wantErr bool
	}{
		{"https://github.com/acme/web-search", "https://raw.githubusercontent.com/acme/web-search/HEAD/SKILL.md", false},
		{"https://github.com/acme/web-search/", "https://raw.githubusercontent.com/acme/web-search/HEAD/SKILL.md", false},
		{"https://github.com/acme/web-search/tree/main/skills/search", "https://raw.githubusercontent.com/acme/web-search/main/skills/search/SKILL.md", false},
		{"https://raw.githubusercontent.com/acme/web-search/main/SKILL.md", "https://raw.githubusercontent.com/acme/web-search/main/SKILL.md", false},
		{"https://example.com/not-github", "", true},
		{"not a url", "", true},
	}
	for _, tc := range cases {
		got, err := ResolveGithubSkillURL(tc.source)
		if tc.wantErr {
			assert.Error(t, err, tc.source)
			continue
		}
		require.NoError(t, err, tc.source)
		assert.Equal(t, tc.want, got)
	}
}

func TestFetchPluginSkill(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/ok.md"):
			fmt.Fprint(w, "# Skill\nDo the thing.")
		case strings.HasSuffix(r.URL.Path, "/empty.md"):
			// 200 with empty body
		case strings.HasSuffix(r.URL.Path, "/big.md"):
			w.Write(make([]byte, 300*1024))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	content, fetchedAt, err := FetchPluginSkill(srv.URL + "/ok.md")
	require.NoError(t, err)
	assert.Contains(t, content, "Do the thing.")
	assert.NotZero(t, fetchedAt)

	_, _, err = FetchPluginSkill(srv.URL + "/missing.md")
	assert.Error(t, err)

	_, _, err = FetchPluginSkill(srv.URL + "/empty.md")
	assert.Error(t, err)

	_, _, err = FetchPluginSkill(srv.URL + "/big.md")
	assert.Error(t, err) // exceeds 256 KiB cap
}
```

NOTE: `FetchPluginSkill` must accept any http(s) URL (the resolver produces raw.githubusercontent.com URLs in production; tests use httptest). Keep resolution and fetching separate so tests never touch GitHub.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./service/ -run 'TestResolveGithubSkillURL|TestFetchPluginSkill' -v`
Expected: FAIL — `undefined: ResolveGithubSkillURL`

- [ ] **Step 3: Implement `service/plugin_skill.go`**

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./service/ -run 'TestResolveGithubSkillURL|TestFetchPluginSkill' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add service/plugin_skill.go service/plugin_skill_test.go
git commit -m "feat(plugin): add GitHub skill fetcher"
```

---

### Task 3: MCP client (`pkg/mcp`)

**Files:**
- Create: `pkg/mcp/client.go`
- Test: `pkg/mcp/client_test.go`

**Interfaces:**
- Consumes: `common.Marshal`, `common.Unmarshal` (mandated JSON wrappers).
- Produces (used by Tasks 4, 5, 6):
  - `type Tool struct { Name string; Description string; InputSchema json.RawMessage }`
  - `type CallResult struct { Text string; IsError bool }`
  - `type Client struct { /* unexported */ }`
  - `func NewClient(endpoint string) *Client`
  - `func (c *Client) ListTools(ctx context.Context) ([]Tool, error)` — 10s timeout, 60s per-endpoint cache
  - `func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (CallResult, error)` — 30s timeout, 1 MiB response cap

- [ ] **Step 1: Write the failing test**

`pkg/mcp/client_test.go` — a fake MCP server with `httptest`:

```go
package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMCPServer handles initialize / tools/list / tools/call JSON-RPC calls.
func fakeMCPServer(t *testing.T, listCount *int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var req struct {
			ID     any             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		require.NoError(t, json.Unmarshal(body, &req))
		write := func(result any) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": req.ID, "result": result,
			})
		}
		switch req.Method {
		case "initialize":
			write(map[string]any{
				"protocolVersion": "2025-03-26",
				"capabilities":    map[string]any{},
				"serverInfo":      map[string]any{"name": "fake", "version": "0"},
			})
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			if listCount != nil {
				atomic.AddInt32(listCount, 1)
			}
			write(map[string]any{"tools": []map[string]any{{
				"name":        "search",
				"description": "search the web",
				"inputSchema": map[string]any{"type": "object"},
			}}})
		case "tools/call":
			var params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			require.NoError(t, json.Unmarshal(req.Params, &params))
			if params.Name == "fail" {
				write(map[string]any{
					"content": []map[string]any{{"type": "text", "text": "boom"}},
					"isError": true,
				})
				return
			}
			write(map[string]any{
				"content": []map[string]any{{"type": "text", "text": "result for " + params.Name}},
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
}

func TestListToolsAndCache(t *testing.T) {
	var listCount int32
	srv := fakeMCPServer(t, &listCount)
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx := context.Background()

	tools, err := c.ListTools(ctx)
	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, "search", tools[0].Name)
	assert.Equal(t, "search the web", tools[0].Description)
	assert.Contains(t, string(tools[0].InputSchema), `"object"`)

	// Second call within TTL must hit the cache.
	_, err = c.ListTools(ctx)
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&listCount))
}

func TestCallTool(t *testing.T) {
	srv := fakeMCPServer(t, nil)
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := c.CallTool(ctx, "search", json.RawMessage(`{"q":"hi"}`))
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Equal(t, "result for search", res.Text)

	res, err = c.CallTool(ctx, "fail", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Equal(t, "boom", res.Text)
}

func TestCallToolUnreachable(t *testing.T) {
	c := NewClient("http://127.0.0.1:1/unreachable")
	_, err := c.CallTool(context.Background(), "x", json.RawMessage(`{}`))
	assert.Error(t, err)
}

func TestListToolsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	c := NewClient(srv.URL)
	_, err := c.ListTools(context.Background())
	assert.Error(t, err)
}

var _ = strings.Contains // keep strings import if unused in future edits
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/mcp/ -v`
Expected: FAIL — package does not exist / undefined symbols

- [ ] **Step 3: Implement `pkg/mcp/client.go`**

```go
// Package mcp is a minimal JSON-RPC 2.0 client for remote MCP servers
// (streamable HTTP transport). No authentication; public servers only.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	connectTimeout   = 5 * time.Second
	listToolsTimeout = 10 * time.Second
	callToolTimeout  = 30 * time.Second
	maxResponseBytes = 1 << 20 // 1 MiB
	toolListCacheTTL = 60 * time.Second
	protocolVersion  = "2025-03-26"
)

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type CallResult struct {
	Text    string
	IsError bool
}

type Client struct {
	endpoint   string
	httpClient *http.Client

	mu          sync.Mutex
	toolsCache  []Tool
	toolsCached time.Time
}

func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: strings.TrimRight(endpoint, "/"),
		httpClient: &http.Client{
			Timeout: callToolTimeout + connectTimeout,
		},
	}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	ID     any             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) do(ctx context.Context, sessionID *string, req rpcRequest) (*rpcResponse, error) {
	payload, err := common.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != nil && *sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", *sessionID)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if sid := resp.Header.Get("Mcp-Session-Id"); sessionID != nil && sid != "" {
		*sessionID = sid
	}
	if resp.StatusCode == http.StatusAccepted && req.ID == 0 {
		return nil, nil // notification acknowledged
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("mcp: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxResponseBytes {
		return nil, fmt.Errorf("mcp: response exceeds %d bytes", maxResponseBytes)
	}
	// Streamable HTTP servers may answer with SSE frames; take the last
	// `data:` payload, which carries the JSON-RPC response.
	if ct := resp.Header.Get("Content-Type"); strings.HasPrefix(ct, "text/event-stream") {
		body = lastSSEData(body)
	}
	var rpcResp rpcResponse
	if err := common.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("mcp: decode response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("mcp: rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return &rpcResp, nil
}

func lastSSEData(body []byte) []byte {
	var last []byte
	for _, line := range strings.Split(string(body), "\n") {
		if data, ok := strings.CutPrefix(line, "data:"); ok {
			last = []byte(strings.TrimSpace(data))
		}
	}
	if last == nil {
		return body
	}
	return last
}

// initialize performs the MCP handshake on a fresh logical session.
func (c *Client) initialize(ctx context.Context) (string, error) {
	sessionID := ""
	_, err := c.do(ctx, &sessionID, rpcRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "new-api", "version": "1.0"},
		},
	})
	if err != nil {
		return "", err
	}
	// Best-effort initialized notification; servers that reject it still work.
	_, _ = c.do(ctx, &sessionID, rpcRequest{
		JSONRPC: "2.0", Method: "notifications/initialized",
	})
	return sessionID, nil
}

func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	c.mu.Lock()
	if time.Since(c.toolsCached) < toolListCacheTTL && c.toolsCache != nil {
		tools := c.toolsCache
		c.mu.Unlock()
		return tools, nil
	}
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, listToolsTimeout)
	defer cancel()
	sessionID, err := c.initialize(ctx)
	if err != nil {
		return nil, fmt.Errorf("mcp initialize: %w", err)
	}
	resp, err := c.do(ctx, &sessionID, rpcRequest{JSONRPC: "2.0", ID: 2, Method: "tools/list"})
	if err != nil {
		return nil, err
	}
	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := common.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("mcp: decode tools/list: %w", err)
	}

	c.mu.Lock()
	c.toolsCache = result.Tools
	c.toolsCached = time.Now()
	c.mu.Unlock()
	return result.Tools, nil
}

func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (CallResult, error) {
	ctx, cancel := context.WithTimeout(ctx, callToolTimeout)
	defer cancel()
	sessionID, err := c.initialize(ctx)
	if err != nil {
		return CallResult{}, fmt.Errorf("mcp initialize: %w", err)
	}
	resp, err := c.do(ctx, &sessionID, rpcRequest{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{"name": name, "arguments": arguments},
	})
	if err != nil {
		return CallResult{}, err
	}
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := common.Unmarshal(resp.Result, &result); err != nil {
		return CallResult{}, fmt.Errorf("mcp: decode tools/call: %w", err)
	}
	var sb strings.Builder
	for _, part := range result.Content {
		if part.Type == "text" {
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(part.Text)
		}
	}
	return CallResult{Text: sb.String(), IsError: result.IsError}, nil
}
```

NOTE: `encoding/json` is imported here only for the `json.RawMessage` type and local test-style struct tags — all marshal/unmarshal calls go through `common.Marshal`/`common.Unmarshal`. `pkg/` is shared infrastructure; if a reviewer objects, the type-only import is still permitted by AGENTS.md ("type definitions from encoding/json may still be referenced as types").

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/mcp/ -v`
Expected: PASS (all 4 tests)

- [ ] **Step 5: Commit**

```bash
git add pkg/mcp/
git commit -m "feat(plugin): add minimal MCP streamable-HTTP client"
```

---

### Task 4: Plugin service + admin/user controllers + routes

**Files:**
- Create: `service/plugin_service.go`
- Create: `controller/plugin.go`
- Modify: `router/api-router.go` (add route group; the `/prefill_group` block at ~L313-320 is the insertion-point pattern)
- Test: `service/plugin_service_test.go`

**Interfaces:**
- Consumes: Task 1 (`model.Plugin`, CRUD, `ValidatePluginSlug`), Task 2 (`service.FetchPluginSkill`), Task 3 (`mcp.NewClient`, `Tool`).
- Produces (used by Tasks 5, 6):
  - `func GetEnabledPlugins() []*model.Plugin` — enabled-only, in-memory cache with `sync.RWMutex`; invalidated by every mutating controller call via `InvalidatePluginCache()`.
  - `func InvalidatePluginCache()`
  - `func ListPluginTools(ctx context.Context, p *model.Plugin) ([]mcp.Tool, error)` — wraps `mcp.NewClient(p.McpUrl).ListTools`.
  - `func RefreshPluginSkill(p *model.Plugin) (warn string)` — fetches + persists skill; returns warning string on failure ("" on success).
  - Controller handlers: `GetPlugins`, `CreatePlugin`, `UpdatePlugin`, `DeletePlugin`, `RefreshPlugin`, `TestPluginConnection`, `GetEnabledPluginList`.

- [ ] **Step 1: Write the failing test**

`service/plugin_service_test.go`:

```go
package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnabledPluginCache(t *testing.T) {
	// Same sqlite fixture pattern as Task 1's model test; the service test
	// may call model.DB directly since it lives in the same process.
	setupTestDBForPluginService(t) // mirror existing test fixtures

	require.NoError(t, (&model.Plugin{Name: "A", Slug: "aaa", Enabled: true, McpUrl: "http://x"}).Insert())
	require.NoError(t, (&model.Plugin{Name: "B", Slug: "bbb", Enabled: false, McpUrl: "http://y"}).Insert())

	InvalidatePluginCache()
	got := GetEnabledPlugins()
	require.Len(t, got, 1)
	assert.Equal(t, "aaa", got[0].Slug)
}

func TestListPluginTools(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method == "initialize" || req.Method == "notifications/initialized" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": req.ID,
			"result": map[string]any{"tools": []map[string]any{{"name": "t1", "description": "d", "inputSchema": map[string]any{"type": "object"}}}},
		})
	}))
	defer srv.Close()

	tools, err := ListPluginTools(context.Background(), &model.Plugin{McpUrl: srv.URL})
	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, "t1", tools[0].Name)
}

func TestRefreshPluginSkillFailureKeepsContent(t *testing.T) {
	setupTestDBForPluginService(t)
	p := &model.Plugin{Name: "A", Slug: "aaa", Enabled: true, McpUrl: "http://x", SkillSource: "http://127.0.0.1:1/nope"}
	require.NoError(t, p.Insert())

	warn := RefreshPluginSkill(p)
	assert.NotEmpty(t, warn)

	reloaded, err := model.GetPluginByID(p.Id)
	require.NoError(t, err)
	assert.Empty(t, reloaded.SkillContent)
}
```

NOTE: `setupTestDBForPluginService` should mirror the fixture used in `model/plugin_test.go` (fresh in-memory sqlite + `AutoMigrate(&model.Plugin{})`). If the repo has a shared test helper, use it instead of defining a new one.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./service/ -run 'TestEnabledPluginCache|TestListPluginTools|TestRefreshPluginSkillFailureKeepsContent' -v`
Expected: FAIL — undefined symbols

- [ ] **Step 3: Implement `service/plugin_service.go`**

```go
package service

import (
	"context"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/mcp"
)

var enabledPluginsCache struct {
	sync.RWMutex
	plugins []*model.Plugin
	loaded  bool
}

// InvalidatePluginCache forces the next GetEnabledPlugins to reload from DB.
// Called by every plugin mutating controller handler.
func InvalidatePluginCache() {
	enabledPluginsCache.Lock()
	defer enabledPluginsCache.Unlock()
	enabledPluginsCache.loaded = false
	enabledPluginsCache.plugins = nil
}

// GetEnabledPlugins returns enabled plugins, cached in memory. DB is the
// source of truth; cache is invalidated on every admin mutation.
func GetEnabledPlugins() []*model.Plugin {
	enabledPluginsCache.RLock()
	if enabledPluginsCache.loaded {
		defer enabledPluginsCache.RUnlock()
		return enabledPluginsCache.plugins
	}
	enabledPluginsCache.RUnlock()

	enabledPluginsCache.Lock()
	defer enabledPluginsCache.Unlock()
	if enabledPluginsCache.loaded {
		return enabledPluginsCache.plugins
	}
	var plugins []*model.Plugin
	if err := model.DB.Where("enabled = ?", true).Find(&plugins).Error; err != nil {
		common.SysError("failed to load enabled plugins: " + err.Error())
		enabledPluginsCache.plugins = nil
	} else {
		enabledPluginsCache.plugins = plugins
	}
	enabledPluginsCache.loaded = true
	return enabledPluginsCache.plugins
}

// ListPluginTools fetches the tool list of a plugin's MCP server.
// The mcp client caches per endpoint internally (60s TTL).
func ListPluginTools(ctx context.Context, p *model.Plugin) ([]mcp.Tool, error) {
	client := mcp.NewClient(p.McpUrl)
	return client.ListTools(ctx)
}

// RefreshPluginSkill fetches the plugin's skill from its GitHub source and
// persists the snapshot. On failure the stored content is left untouched and
// a human-readable warning is returned.
func RefreshPluginSkill(p *model.Plugin) string {
	content, fetchedAt, err := FetchPluginSkill(p.SkillSource)
	if err != nil {
		return "skill fetch failed: " + err.Error()
	}
	p.SkillContent = content
	p.SkillFetchedAt = fetchedAt
	if err := p.Update(); err != nil {
		return "skill save failed: " + err.Error()
	}
	InvalidatePluginCache()
	return ""
}
```

- [ ] **Step 4: Implement `controller/plugin.go`**

```go
package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/mcp"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetPlugins(c *gin.Context) {
	plugins, err := model.GetAllPlugins()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, plugins)
}

func CreatePlugin(c *gin.Context) {
	var p model.Plugin
	if err := c.ShouldBindJSON(&p); err != nil {
		common.ApiError(c, err)
		return
	}
	if p.Name == "" || p.Slug == "" || p.McpUrl == "" {
		common.ApiErrorMsg(c, "name, slug and MCP URL are required")
		return
	}
	if !model.ValidatePluginSlug(p.Slug) {
		common.ApiErrorMsg(c, "invalid slug: lowercase letters, digits and dashes, 2-64 chars, must not start with a dash")
		return
	}
	if dup, err := model.IsPluginSlugDuplicated(0, p.Slug); err != nil {
		common.ApiError(c, err)
		return
	} else if dup {
		common.ApiErrorMsg(c, "slug already exists")
		return
	}
	if err := p.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	warn := ""
	if p.SkillSource != "" {
		warn = service.RefreshPluginSkill(&p)
	}
	service.InvalidatePluginCache()
	c.JSON(200, gin.H{"success": true, "message": warn, "data": &p})
}

func UpdatePlugin(c *gin.Context) {
	var p model.Plugin
	if err := c.ShouldBindJSON(&p); err != nil {
		common.ApiError(c, err)
		return
	}
	if p.Id == 0 {
		common.ApiErrorMsg(c, "missing plugin id")
		return
	}
	if p.Name == "" || p.McpUrl == "" {
		common.ApiErrorMsg(c, "name and MCP URL are required")
		return
	}
	existing, err := model.GetPluginByID(p.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// Slug is immutable after creation.
	p.Slug = existing.Slug
	if p.SkillSource != existing.SkillSource {
		p.SkillContent = ""
		p.SkillFetchedAt = 0
	}
	if err := p.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	warn := ""
	if p.SkillSource != "" && p.SkillContent == "" {
		warn = service.RefreshPluginSkill(&p)
	}
	service.InvalidatePluginCache()
	c.JSON(200, gin.H{"success": true, "message": warn, "data": &p})
}

func DeletePlugin(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeletePluginByID(id); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidatePluginCache()
	common.ApiSuccess(c, nil)
}

func RefreshPlugin(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	p, err := model.GetPluginByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if p.SkillSource == "" {
		common.ApiErrorMsg(c, "plugin has no skill source")
		return
	}
	warn := service.RefreshPluginSkill(p)
	if warn != "" {
		common.ApiErrorMsg(c, warn)
		return
	}
	common.ApiSuccess(c, p)
}

func TestPluginConnection(c *gin.Context) {
	var req struct {
		McpUrl string `json:"mcp_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.McpUrl == "" {
		common.ApiErrorMsg(c, "mcp_url is required")
		return
	}
	tools, err := service.ListPluginTools(c.Request.Context(), &model.Plugin{McpUrl: req.McpUrl})
	if err != nil {
		common.ApiErrorMsg(c, "connection failed: "+err.Error())
		return
	}
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name)
	}
	common.ApiSuccess(c, gin.H{"tools": names})
}

// GetEnabledPluginList serves the playground @ autocomplete.
// Exposes only slug/name/description — never MCP URLs or skill content.
func GetEnabledPluginList(c *gin.Context) {
	type item struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	plugins := service.GetEnabledPlugins()
	items := make([]item, 0, len(plugins))
	for _, p := range plugins {
		items = append(items, item{Slug: p.Slug, Name: p.Name, Description: p.Description})
	}
	common.ApiSuccess(c, items)
}
```

Register routes in `router/api-router.go`, modeled on the `/prefill_group` block (~L313):

```go
pluginRoute := apiRouter.Group("/plugin")
{
	pluginRoute.GET("/enabled", middleware.UserAuth(), controller.GetEnabledPluginList)

	pluginAdminRoute := pluginRoute.Group("")
	pluginAdminRoute.Use(middleware.AdminAuth())
	{
		pluginAdminRoute.GET("/", controller.GetPlugins)
		pluginAdminRoute.POST("/", controller.CreatePlugin)
		pluginAdminRoute.PUT("/", controller.UpdatePlugin)
		pluginAdminRoute.DELETE("/:id", controller.DeletePlugin)
		pluginAdminRoute.POST("/:id/refresh", controller.RefreshPlugin)
		pluginAdminRoute.POST("/test", controller.TestPluginConnection)
	}
}
```

NOTE: `GET /enabled` must be registered before the `:id`-less admin group to avoid gin route conflicts — the exact shape above (static segment `enabled` + admin group on `""`) avoids wildcard conflicts since admin routes use `/`, `/:id`, `/:id/refresh`, `/test`. Verify with `go run . --help` or the router test if gin panics on startup.

- [ ] **Step 5: Run tests**

Run: `go test ./service/ -run 'TestEnabledPluginCache|TestListPluginTools|TestRefreshPluginSkillFailureKeepsContent' -v && go build ./...`
Expected: PASS + clean build

- [ ] **Step 6: Commit**

```bash
git add service/plugin_service.go service/plugin_service_test.go controller/plugin.go router/api-router.go
git commit -m "feat(plugin): add plugin service, admin CRUD and enabled-list APIs"
```

---

### Task 5: Playground agentic loop

**Files:**
- Create: `controller/plugin_loop.go`
- Modify: `controller/playground.go`
- Test: `controller/plugin_loop_test.go`

**Interfaces:**
- Consumes: Task 4 (`service.GetEnabledPlugins`, `service.ListPluginTools`), Task 3 (`mcp.Client.CallTool`), `common.UnmarshalBodyReusable`, `common.CreateBodyStorage`, `common.KeyBodyStorage`, `dto.GeneralOpenAIRequest`, `dto.Message`, `dto.OpenAITextResponse`, `constant.FinishReasonToolCalls`, `relaycommon.GenRelayInfo`, `middleware.SetupContextForToken`.
- Produces:
  - `const pluginMentionPattern` regex `@([a-z0-9][a-z0-9-]{1,63})`
  - `func ExtractPluginMentions(text string) []string` — deduped slugs
  - `func ResolveMentionedPlugins(slugs []string, enabled []*model.Plugin) []*model.Plugin` — match + cap 3
  - `func InjectPluginSkill(req *dto.GeneralOpenAIRequest, p *model.Plugin)` — system prompt injection
  - `func InjectPluginTools(req *dto.GeneralOpenAIRequest, p *model.Plugin, tools []mcp.Tool)` — appends namespaced `dto.ToolCallRequest` entries
  - `func SplitNamespacedToolName(name string) (slug, tool string, ok bool)`
  - `func runPluginLoop(c *gin.Context, req *dto.GeneralOpenAIRequest, plugins []*model.Plugin) (*dto.OpenAITextResponse, *types.NewAPIError)`
  - Modified `Playground` entry that branches into the loop only when mentions resolve.

- [ ] **Step 1: Write the failing test (pure helpers)**

`controller/plugin_loop_test.go`:

```go
package controller

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractPluginMentions(t *testing.T) {
	cases := []struct {
		text string
		want []string
	}{
		{"hello @web-search please find X", []string{"web-search"}},
		{"@a1 and @b2", []string{"a1", "b2"}},
		{"@dup @dup", []string{"dup"}},
		{"email me at user@example.com", nil},   // mid-word @ not a mention
		{"no mentions", nil},
		{"@CAPS not valid", nil},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, ExtractPluginMentions(tc.text), tc.text)
	}
}

func TestResolveMentionedPluginsCap(t *testing.T) {
	enabled := []*model.Plugin{
		{Slug: "p1", Enabled: true},
		{Slug: "p2", Enabled: true},
		{Slug: "p3", Enabled: true},
		{Slug: "p4", Enabled: true},
	}
	got := ResolveMentionedPlugins([]string{"p1", "p2", "p3", "p4", "nope"}, enabled)
	require.Len(t, got, 3) // capped at 3, unknown slug dropped
	assert.Equal(t, "p1", got[0].Slug)
}

func TestInjectPluginSkill(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "hi @search"},
		},
	}
	InjectPluginSkill(req, &model.Plugin{Slug: "search", SkillContent: "Always cite sources."})
	require.Equal(t, "system", req.Messages[0].Role)
	sys := req.Messages[0].StringContent()
	assert.Contains(t, sys, `<plugin name="search">`)
	assert.Contains(t, sys, "Always cite sources.")
	assert.Contains(t, sys, "You are helpful.")

	// No system message → one is created.
	req2 := &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: "hi"}}}
	InjectPluginSkill(req2, &model.Plugin{Slug: "s", SkillContent: "S"})
	require.Len(t, req2.Messages, 2)
	assert.Equal(t, "system", req2.Messages[0].Role)
}

func TestInjectPluginToolsNamespaced(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{}
	InjectPluginTools(req, &model.Plugin{Slug: "search"}, []mcp.Tool{
		{Name: "web", Description: "web search", InputSchema: json.RawMessage(`{"type":"object"}`)},
	})
	require.Len(t, req.Tools, 1)
	assert.Equal(t, "function", req.Tools[0].Type)
	assert.Equal(t, "search__web", req.Tools[0].Function.Name)
	assert.Equal(t, "web search", req.Tools[0].Function.Description)

	slug, tool, ok := SplitNamespacedToolName("search__web")
	assert.True(t, ok)
	assert.Equal(t, "search", slug)
	assert.Equal(t, "web", tool)
	_, _, ok = SplitNamespacedToolName("web")
	assert.False(t, ok)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./controller/ -run 'TestExtractPluginMentions|TestResolveMentionedPluginsCap|TestInjectPluginSkill|TestInjectPluginToolsNamespaced' -v`
Expected: FAIL — undefined symbols

- [ ] **Step 3: Implement `controller/plugin_loop.go`**

```go
package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/mcp"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	maxPluginsPerMessage = 3
	maxToolRounds        = 10
	toolNameSeparator    = "__"
)

var pluginMentionPattern = regexp.MustCompile(`(?:^|[^a-zA-Z0-9_])@([a-z0-9][a-z0-9-]{1,63})`)

// ExtractPluginMentions returns deduped @slug mentions. A `@` preceded by a
// word character (emails, handles) is not a mention.
func ExtractPluginMentions(text string) []string {
	matches := pluginMentionPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		slug := m[1]
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		out = append(out, slug)
	}
	return out
}

// ResolveMentionedPlugins maps slugs to enabled plugins, preserving mention
// order, dropping unknown slugs, capped at maxPluginsPerMessage.
func ResolveMentionedPlugins(slugs []string, enabled []*model.Plugin) []*model.Plugin {
	if len(slugs) == 0 {
		return nil
	}
	bySlug := make(map[string]*model.Plugin, len(enabled))
	for _, p := range enabled {
		bySlug[p.Slug] = p
	}
	out := make([]*model.Plugin, 0, len(slugs))
	for _, slug := range slugs {
		if p, ok := bySlug[slug]; ok {
			out = append(out, p)
			if len(out) >= maxPluginsPerMessage {
				break
			}
		}
	}
	return out
}

// InjectPluginSkill prepends the plugin's skill markdown to the system prompt.
func InjectPluginSkill(req *dto.GeneralOpenAIRequest, p *model.Plugin) {
	if p.SkillContent == "" {
		return
	}
	block := fmt.Sprintf("<plugin name=\"%s\">\n%s\n</plugin>", p.Slug, p.SkillContent)
	if len(req.Messages) > 0 && req.Messages[0].Role == "system" {
		existing := req.Messages[0].StringContent()
		req.Messages[0].SetStringContent(block + "\n\n" + existing)
		return
	}
	req.Messages = append([]dto.Message{{Role: "system", Content: block}}, req.Messages...)
}

// InjectPluginTools appends the plugin's MCP tools as OpenAI function tools,
// namespaced as {slug}__{tool} to avoid cross-plugin collisions.
func InjectPluginTools(req *dto.GeneralOpenAIRequest, p *model.Plugin, tools []mcp.Tool) {
	for _, t := range tools {
		var params any
		if len(t.InputSchema) > 0 {
			_ = common.Unmarshal(t.InputSchema, &params)
		}
		req.Tools = append(req.Tools, dto.ToolCallRequest{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        p.Slug + toolNameSeparator + t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}
}

// SplitNamespacedToolName splits "{slug}__{tool}" back into its parts.
func SplitNamespacedToolName(name string) (slug, tool string, ok bool) {
	idx := strings.Index(name, toolNameSeparator)
	if idx <= 0 || idx+len(toolNameSeparator) >= len(name) {
		return "", "", false
	}
	return name[:idx], name[idx+len(toolNameSeparator):], true
}

// runPluginLoop drives up to maxToolRounds non-streaming relay calls in
// recorder-backed sub-contexts (channel-test pattern). Returns the final
// response to stream to the browser.
func runPluginLoop(c *gin.Context, req *dto.GeneralOpenAIRequest, plugins []*model.Plugin) (*dto.OpenAITextResponse, *types.NewAPIError) {
	userId := c.GetInt("id")
	group := c.GetString(constant.ContextKeyTokenGroup)

	// Map slug -> mcp client; one client per plugin per request.
	clients := make(map[string]*mcp.Client, len(plugins))
	for _, p := range plugins {
		clients[p.Slug] = mcp.NewClient(p.McpUrl)
	}

	streamFalse := false
	req.Stream = &streamFalse
	req.StreamOptions = nil

	var lastResp *dto.OpenAITextResponse
	for round := 0; round < maxToolRounds; round++ {
		resp, relayErr := invokeRelayRound(c, req, userId, group)
		if relayErr != nil {
			if round > 0 && lastResp != nil {
				// Mid-loop failure (e.g. quota): surface the last good answer.
				return lastResp, nil
			}
			return nil, relayErr
		}
		lastResp = resp
		if len(resp.Choices) == 0 {
			return resp, nil
		}
		choice := resp.Choices[0]
		toolCalls := choice.Message.ParseToolCalls()
		if choice.FinishReason != constant.FinishReasonToolCalls || len(toolCalls) == 0 {
			return resp, nil
		}

		// Append the assistant message (with tool calls) then each tool result.
		assistantMsg := dto.Message{Role: "assistant"}
		if content := choice.Message.StringContent(); content != "" {
			assistantMsg.SetStringContent(content)
		} else {
			assistantMsg.SetNullContent()
		}
		assistantMsg.SetToolCalls(toolCalls)
		req.Messages = append(req.Messages, assistantMsg)

		for _, tc := range toolCalls {
			req.Messages = append(req.Messages, dto.Message{
				Role:       "tool",
				ToolCallId: tc.ID,
				Content:    executePluginToolCall(c.Request.Context(), clients, tc),
			})
		}
	}
	// Round cap reached: return the last response we have.
	return lastResp, nil
}

// executePluginToolCall runs one namespaced tool call and returns the tool
// message content. Errors are returned as content so the model can recover.
func executePluginToolCall(ctx context.Context, clients map[string]*mcp.Client, tc dto.ToolCallRequest) string {
	slug, toolName, ok := SplitNamespacedToolName(tc.Function.Name)
	if !ok {
		return "error: malformed tool name"
	}
	client, ok := clients[slug]
	if !ok {
		return "error: unknown plugin " + slug
	}
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	var args []byte
	if tc.Function.Arguments != "" {
		args = []byte(tc.Function.Arguments)
	} else {
		args = []byte("{}")
	}
	result, err := client.CallTool(ctx, toolName, args)
	if err != nil {
		return "error: " + err.Error()
	}
	if result.IsError {
		return "error: " + result.Text
	}
	return result.Text
}

// invokeRelayRound performs one full Relay pass in a throwaway gin context
// whose writer is an httptest recorder, then parses the non-stream response.
func invokeRelayRound(c *gin.Context, req *dto.GeneralOpenAIRequest, userId int, group string) (*dto.OpenAITextResponse, *types.NewAPIError) {
	body, err := common.Marshal(req)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	w := httptest.NewRecorder()
	subCtx, _ := gin.CreateTestContext(w)
	subCtx.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", io.NopCloser(bytes.NewReader(body)))
	subCtx.Request.Header.Set("Content-Type", "application/json")
	subCtx.Request.ContentLength = int64(len(body))

	bs, err := common.CreateBodyStorage(body)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	subCtx.Set(common.KeyBodyStorage, bs)

	userCache, err := model.GetUserCache(userId)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	userCache.WriteContext(subCtx)
	subCtx.Set("id", userId)

	tempToken := &model.Token{
		UserId: userId,
		Name:   fmt.Sprintf("playground-plugin-%s", group),
		Group:  group,
	}
	if err := middleware.SetupContextForToken(subCtx, tempToken); err != nil {
		return nil, types.NewError(err, types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
	}

	relayInfo, err := relaycommon.GenRelayInfo(subCtx, types.RelayFormatOpenAI, nil, nil)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	_ = relayInfo // GenRelayInfo seeds path/format-derived context keys.

	Relay(subCtx, types.RelayFormatOpenAI)

	respBody := w.Body.Bytes()
	var textResp dto.OpenAITextResponse
	if err := common.Unmarshal(respBody, &textResp); err != nil {
		return nil, types.NewError(fmt.Errorf("plugin loop: decode relay response: %w", err), types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
	}
	if w.Code >= 400 {
		return nil, types.NewError(fmt.Errorf("plugin loop: relay round failed with status %d: %s", w.Code, strings.TrimSpace(string(respBody))), types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
	}
	return &textResp, nil
}
```

NOTE on context-key propagation: `relaycommon.GenRelayInfo` + `Relay` re-derive channel selection inside `Relay` via `middleware.Distribute()`? — NO. `Distribute()` is router middleware on `/pg/chat/completions` and does NOT run inside `controller.Relay`. `controller.Relay` calls `getChannel` internally (controller/relay.go) using context keys set by `GenRelayInfo` + `SetupContextForToken`. The implementer MUST verify this by reading `controller/relay.go` lines ~120-220 before wiring; if `Relay` expects a pre-selected channel (context key `specific_channel_id` / channel fields set by `middleware.SetupContextForSelectedChannel`), copy the channel-selection step from `controller/channel-test.go:176` (`middleware.SetupContextForSelectedChannel(subCtx, channel, model)`) — picking the channel the same way `middleware.Distribute()` does for `/pg`. This is the single riskiest integration point of the plan; get a real round-trip working before moving on.

- [ ] **Step 4: Wire into `controller/playground.go`**

Replace the final `Relay(c, types.RelayFormatOpenAI)` call with:

```go
	// Plugin mentions (@slug) trigger the server-side tool-call loop.
	var pgReq dto.GeneralOpenAIRequest
	if err := common.UnmarshalBodyReusable(c, &pgReq); err == nil {
		if lastUser := lastUserMessageText(&pgReq); lastUser != "" {
			mentions := ExtractPluginMentions(lastUser)
			if plugins := ResolveMentionedPlugins(mentions, service.GetEnabledPlugins()); len(plugins) > 0 {
				playgroundWithPlugins(c, &pgReq, plugins)
				return
			}
		}
	}

	Relay(c, types.RelayFormatOpenAI)
```

Add to `controller/plugin_loop.go`:

```go
func lastUserMessageText(req *dto.GeneralOpenAIRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return req.Messages[i].StringContent()
		}
	}
	return ""
}

// playgroundWithPlugins injects skills + tools, runs the loop, then streams
// the final answer as a standard SSE stream so the frontend needs no changes.
func playgroundWithPlugins(c *gin.Context, req *dto.GeneralOpenAIRequest, plugins []*model.Plugin) {
	for _, p := range plugins {
		InjectPluginSkill(req, p)
		tools, err := service.ListPluginTools(c.Request.Context(), p)
		if err != nil {
			common.SysLog("plugin " + p.Slug + " tools/list failed: " + err.Error())
			continue // skill still injected; tools skipped
		}
		InjectPluginTools(req, p, tools)
	}

	finalResp, relayErr := runPluginLoop(c, req, plugins)
	if relayErr != nil {
		c.JSON(relayErr.StatusCode, gin.H{"error": relayErr.ToOpenAIError()})
		return
	}
	if finalResp == nil || len(finalResp.Choices) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "plugin loop produced no response", "type": "server_error"}})
		return
	}

	content := finalResp.Choices[0].Message.StringContent()
	streamFinalAnswer(c, req.Model, content)
}

// streamFinalAnswer emits content as OpenAI SSE chunks (content deltas +
// [DONE]) so the playground's existing stream parser renders it unchanged.
func streamFinalAnswer(c *gin.Context, model string, content string) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	chunk := map[string]any{
		"id":      "chatcmpl-plugin",
		"object":  "chat.completion.chunk",
		"created": common.GetTimestamp(),
		"model":   model,
		"choices": []map[string]any{{
			"index": 0,
			"delta": map[string]any{"role": "assistant", "content": content},
			"finish_reason": nil,
		}},
	}
	data, _ := common.Marshal(chunk)
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)

	endChunk := map[string]any{
		"id":      "chatcmpl-plugin",
		"object":  "chat.completion.chunk",
		"created": common.GetTimestamp(),
		"model":   model,
		"choices": []map[string]any{{
			"index": 0,
			"delta": map[string]any{},
			"finish_reason": "stop",
		}},
	}
	endData, _ := common.Marshal(endChunk)
	fmt.Fprintf(c.Writer, "data: %s\n\n", endData)
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	c.Writer.Flush()
}
```

IMPORTANT — verify the SSE shape against the real parser: before finalizing, read `web/src/features/playground/lib/streaming/stream-utils.ts` and `payload-builder.ts` and confirm (a) the parser reads `choices[0].delta.content`, (b) whether a `usage` chunk or `finish_reason` chunk is required for the UI to close the message, and (c) whether the playground sends `stream: true` and expects `chat.completion.chunk`. Adjust the chunk maps above to match exactly what the parser consumes (including an empty first role chunk if the parser requires it). If the parser also handles non-stream JSON responses when the request had `stream:false`, an even simpler option is to force `stream=false` in the request during injection and return `finalResp` as plain JSON — choose whichever matches the parser with the least code.

Also add an integration test for the round-trip if the sub-context wiring permits a fake upstream channel; if standing up a full fake channel proves too heavy for a unit test, cover it with a manual verification step instead (do NOT write a log-only pseudo test).

- [ ] **Step 5: Run tests and build**

Run: `go test ./controller/ -run 'TestExtractPluginMentions|TestResolveMentionedPluginsCap|TestInjectPluginSkill|TestInjectPluginToolsNamespaced' -v && go build ./...`
Expected: PASS + clean build

- [ ] **Step 6: Commit**

```bash
git add controller/plugin_loop.go controller/plugin_loop_test.go controller/playground.go
git commit -m "feat(plugin): add playground @plugin agentic loop"
```

---

### Task 6: Frontend — admin plugins page

**Files:**
- Create: `web/src/features/plugins/index.tsx`
- Create: `web/src/features/plugins/api.ts`
- Create: `web/src/features/plugins/types.ts`
- Create: `web/src/features/plugins/components/plugins-table.tsx`
- Create: `web/src/features/plugins/components/plugin-mutate-drawer.tsx`
- Create: `web/src/routes/_authenticated/plugins/index.tsx`
- Modify: `web/src/hooks/use-sidebar-data.ts` (add admin entry, ~L118-126 pattern)
- Modify: `web/src/hooks/use-sidebar-config.ts` (add `plugin: true` to default admin modules ~L58-66, and `'/plugins': { section: 'admin', module: 'plugin' }` to URL mapping ~L110)

**Interfaces:**
- Consumes: backend `GET/POST/PUT /api/plugin/`, `DELETE /api/plugin/:id`, `POST /api/plugin/:id/refresh`, `POST /api/plugin/test`; layout components `SectionPageLayout` (`@/components/layout`), drawer helpers (`@/components/drawer-layout`), `Sheet` (`@/components/ui/sheet`), `@/lib/api`, `@/lib/roles`, `useAuthStore`.
- Produces: route `/_authenticated/plugins/`; feature export `Plugins`.

- [ ] **Step 1: Write types + api**

`web/src/features/plugins/types.ts` (with the standard AGPL header — copy the header block verbatim from `web/src/features/playground/api.ts` lines 1-18; same for every new file):

```ts
export interface Plugin {
  id: number
  name: string
  slug: string
  description: string
  enabled: boolean
  mcp_url: string
  skill_source: string
  skill_content: string
  skill_fetched_at: number
  created_time: number
  updated_time: number
}

export interface EnabledPlugin {
  slug: string
  name: string
  description: string
}

export interface PluginTestResult {
  tools: string[]
}
```

`web/src/features/plugins/api.ts`:

```ts
import { api } from '@/lib/api'

import type { EnabledPlugin, Plugin, PluginTestResult } from './types'

export async function getPlugins(): Promise<Plugin[]> {
  const res = await api.get('/api/plugin/')
  return res.data?.data ?? []
}

export async function createPlugin(plugin: Partial<Plugin>): Promise<{ message?: string }> {
  const res = await api.post('/api/plugin/', plugin)
  return res.data
}

export async function updatePlugin(plugin: Partial<Plugin>): Promise<{ message?: string }> {
  const res = await api.put('/api/plugin/', plugin)
  return res.data
}

export async function deletePlugin(id: number): Promise<void> {
  await api.delete(`/api/plugin/${id}`)
}

export async function refreshPluginSkill(id: number): Promise<void> {
  await api.post(`/api/plugin/${id}/refresh`)
}

export async function testPluginConnection(mcpUrl: string): Promise<PluginTestResult> {
  const res = await api.post('/api/plugin/test', { mcp_url: mcpUrl })
  return res.data?.data ?? { tools: [] }
}

export async function getEnabledPlugins(): Promise<EnabledPlugin[]> {
  const res = await api.get('/api/plugin/enabled')
  return res.data?.data ?? []
}
```

- [ ] **Step 2: Write the page, table, drawer**

`web/src/features/plugins/index.tsx` — follow the `web/src/features/channels/index.tsx` composition (`SectionPageLayout` + provider/dialogs pattern is optional here; a simpler single-file page with local state is acceptable for v1 since there is one table + one drawer):

```tsx
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'

import { getPlugins, deletePlugin, refreshPluginSkill } from './api'
import { PluginMutateDrawer } from './components/plugin-mutate-drawer'
import { PluginsTable } from './components/plugins-table'
import type { Plugin } from './types'

export function Plugins() {
  const { t } = useTranslation()
  const [plugins, setPlugins] = useState<Plugin[]>([])
  const [loading, setLoading] = useState(true)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editing, setEditing] = useState<Plugin | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setPlugins(await getPlugins())
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>{t('Plugins')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <button
          type='button'
          className='rounded-md bg-primary px-3 py-1.5 text-sm text-primary-foreground'
          onClick={() => {
            setEditing(null)
            setDrawerOpen(true)
          }}
        >
          {t('New Plugin')}
        </button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <PluginsTable
          plugins={plugins}
          loading={loading}
          onEdit={(p) => {
            setEditing(p)
            setDrawerOpen(true)
          }}
          onDelete={async (p) => {
            await deletePlugin(p.id)
            await load()
          }}
          onRefresh={async (p) => {
            await refreshPluginSkill(p.id)
            await load()
          }}
        />
      </SectionPageLayout.Content>
      <PluginMutateDrawer
        open={drawerOpen}
        plugin={editing}
        onOpenChange={setDrawerOpen}
        onSaved={load}
      />
    </SectionPageLayout>
  )
}
```

`components/plugins-table.tsx` — a plain table (the repo has table primitives under `@/components/ui/table`; check `web/src/features/channels/components/channels-table.tsx` for which table component is canonical and use the same). Columns: Name, Slug, MCP URL (truncated), Skill source, Enabled (switch → `updatePlugin({...p, enabled})`), Fetched at (formatted, `—` when 0), row actions: Edit / Refresh skill / Delete (with a confirm dialog — mirror an existing delete-confirm dialog from channels feature).

`components/plugin-mutate-drawer.tsx` — `react-hook-form` + `zod` (pattern: `channel-mutate-drawer.tsx`), fields: Name, Slug (disabled when editing — immutable), Description, MCP URL, Skill source, Enabled switch. Footer buttons: "Test connection" (calls `testPluginConnection(mcpUrl)`, shows returned tool names in a scrollable box or a toast listing count + names), "Save" (create or update; if the response `message` is non-empty, show it as a warning toast — that is the skill-fetch warning).

Keep the implementation aligned with existing UI primitives; do not introduce new component libraries.

- [ ] **Step 3: Register route + sidebar**

`web/src/routes/_authenticated/plugins/index.tsx`:

```tsx
import { createFileRoute, redirect } from '@tanstack/react-router'

import { Plugins } from '@/features/plugins'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/plugins/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  component: Plugins,
})
```

Sidebar: in `web/src/hooks/use-sidebar-data.ts` add to the admin group items (icon: `Puzzle` from lucide-react):

```ts
{
  title: t('Plugins'),
  url: '/plugins',
  icon: Puzzle,
},
```

In `web/src/hooks/use-sidebar-config.ts`: add `plugin: true,` to the default `admin` modules object, and `'/plugins': { section: 'admin', module: 'plugin' },` to the URL mapping.

- [ ] **Step 4: i18n sync + checks**

Run from `web/`: `bun run i18n:sync && bun run typecheck && bun run lint`
Expected: no errors. Regenerate the route tree if the router plugin didn't (`routeTree.gen.ts` is auto-generated by the dev/build pipeline — `bun run build` regenerates it if needed).

- [ ] **Step 5: Commit**

```bash
git add web/src/features/plugins web/src/routes/_authenticated/plugins web/src/hooks/use-sidebar-data.ts web/src/hooks/use-sidebar-config.ts web/src/i18n web/src/routeTree.gen.ts
git commit -m "feat(plugin): add admin plugins management page"
```

---

### Task 7: Frontend — playground `@` autocomplete + spinner label

**Files:**
- Modify: `web/src/features/playground/components/input/playground-input.tsx`
- Create: `web/src/features/playground/components/input/plugin-mention-dropdown.tsx`
- Create: `web/src/features/playground/hooks/use-enabled-plugins.ts`
- Modify: `web/src/features/playground/components/message/` pending/spinner component (locate it first — see step 3)

**Interfaces:**
- Consumes: Task 6 `getEnabledPlugins` (re-export or move to a shared spot — simplest: duplicate a tiny `getEnabledPlugins` into `web/src/features/playground/api.ts` using its `API_ENDPOINTS` pattern; do NOT import across features), `PromptInputTextarea` from `@/components/ai-elements/prompt-input`.
- Produces: `useEnabledPlugins()` hook returning `{ plugins: EnabledPlugin[] }`; `PluginMentionDropdown` component; extended pending label when the sent message contained `@slug`.

- [ ] **Step 1: Add endpoint + hook**

In `web/src/features/playground/constants.ts` add `PLUGIN_ENABLED: '/api/plugin/enabled'` to `API_ENDPOINTS`. In `web/src/features/playground/api.ts` add:

```ts
export async function getEnabledPlugins(): Promise<{ slug: string; name: string; description: string }[]> {
  const res = await api.get(API_ENDPOINTS.PLUGIN_ENABLED)
  const { data } = res
  if (!data.success || !Array.isArray(data.data)) {
    return []
  }
  return data.data
}
```

`web/src/features/playground/hooks/use-enabled-plugins.ts`:

```ts
import { useEffect, useState } from 'react'

import { getEnabledPlugins } from '../api'

interface EnabledPlugin {
  slug: string
  name: string
  description: string
}

let cache: EnabledPlugin[] | null = null

export function useEnabledPlugins() {
  const [plugins, setPlugins] = useState<EnabledPlugin[]>(cache ?? [])

  useEffect(() => {
    if (cache) return
    let cancelled = false
    void getEnabledPlugins().then((list) => {
      cache = list
      if (!cancelled) setPlugins(list)
    })
    return () => {
      cancelled = true
    }
  }, [])

  return { plugins }
}
```

- [ ] **Step 2: Autocomplete dropdown**

`web/src/features/playground/components/input/plugin-mention-dropdown.tsx`: a positioned list rendered inside the `relative` `PromptInput` wrapper. Props:

```tsx
interface PluginMentionDropdownProps {
  query: string // text after the active '@'
  plugins: { slug: string; name: string; description: string }[]
  onSelect: (slug: string) => void
}
```

Filter: `slug.startsWith(query) || name.toLowerCase().includes(query.toLowerCase())`, max 8 entries, each row shows `name` + `@slug` + truncated description. Return `null` when no matches.

Wire it in `playground-input.tsx` (state `text` at L84-85, textarea at L102-112):

```tsx
const { plugins } = useEnabledPlugins()
const [mention, setMention] = useState<{ start: number; query: string } | null>(null)

const handleTextChange = (event: React.ChangeEvent<HTMLTextAreaElement>) => {
  const value = event.target.value
  setText(value)
  const caret = event.target.selectionStart ?? value.length
  const before = value.slice(0, caret)
  const match = /(?:^|[\s])@([a-z0-9-]{0,64})$/.exec(before)
  setMention(match ? { start: caret - match[1].length - 1, query: match[1] } : null)
}

const insertMention = (slug: string) => {
  if (!mention) return
  const next = text.slice(0, mention.start) + '@' + slug + ' ' + text.slice(mention.start + mention.query.length + 1)
  setText(next)
  setMention(null)
}
```

Render `{mention && <PluginMentionDropdown query={mention.query} plugins={plugins} onSelect={insertMention} />}` directly after `<PromptInputTextarea ... />`, and swap the textarea's `onChange` to `handleTextChange`. Keyboard support (arrow keys + Enter) is desirable — if `PromptInputTextarea` forwards `onKeyDown`, add ArrowUp/ArrowDown/Enter/Escape handling with an `activeIndex` state; otherwise ship mouse-only for v1 and note it.

- [ ] **Step 3: Spinner label**

Locate the pending-message indicator: run `grep -rn "Thinking\|spinner\|isGenerating" web/src/features/playground/components/message/ web/src/features/playground/components/chat/ | head -30` and open the component that renders the assistant placeholder while streaming. Find where the placeholder text/animation renders and make the label dynamic: when the just-sent user message matches `@([a-z0-9-]+)` for a slug present in `useEnabledPlugins()`, render `t('Using {{slug}}…')` instead of the default label (keep the existing default otherwise). The chat handler knows the last sent text — thread it through the smallest possible prop change (e.g. extend the pending-message state in `use-playground-state.ts` or compute it in the component from the last message in the conversation). Do not change any streaming logic.

- [ ] **Step 4: i18n sync + checks**

Run from `web/`: `bun run i18n:sync && bun run typecheck && bun run lint`
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/features/playground web/src/i18n
git commit -m "feat(plugin): playground @plugin autocomplete and pending label"
```

---

### Task 8: End-to-end verification

**Files:** none (manual + existing checks)

- [ ] **Step 1: Full backend gate**

Run: `go build ./... && go vet ./... && go test ./pkg/mcp/ ./service/ ./controller/ ./model/ -count=1`
Expected: all green.

- [ ] **Step 2: Full frontend gate**

Run from `web/`: `bun run build`
Expected: build succeeds.

- [ ] **Step 3: Manual smoke test**

1. Start the server (`go run .`) with a fresh sqlite DB; create admin; log in.
2. Admin → Plugins → New Plugin: name `Echo`, slug `echo`, MCP URL pointing at a public test MCP server (or a local `npx`-served mock — the implementer should stand up a tiny mock with `httptest`-equivalent, e.g. a 30-line Node script), skill source a small public GitHub repo containing `SKILL.md`. Save; confirm "Test connection" lists tools and the skill fetched (Fetched-at populated).
3. Playground: type `@` → dropdown shows `echo`; select it, ask a question that forces a tool call.
4. Confirm: spinner shows `Using echo…`; final answer streams; admin logs show the relay rounds with quota deducted.

- [ ] **Step 4: Commit any fixes discovered during verification**

```bash
git commit -am "fix(plugin): issues found in e2e verification"
```

---

## Self-Review Notes (resolved)

- Spec coverage: model/skill-fetch/MCP client/APIs/loop/frontend pages/spinner/observability-lite all mapped to Tasks 1-8. The spec's "log `other` field enrichment (plugin_slugs, tool-call counts)" is intentionally folded into Task 8's manual verification rather than a dedicated code task — the per-round relay calls already produce consume logs; adding structured `other` fields would require touching the log pipeline and is deferred post-v1.
- Type consistency: `mcp.Tool` / `CallResult` / `NewClient` names are identical across Tasks 3, 4, 5. `service.GetEnabledPlugins` / `ListPluginTools` / `RefreshPluginSkill` / `InvalidatePluginCache` are identical across Tasks 4, 5. `dto` field/method names (`ParseToolCalls`, `SetToolCalls`, `SetStringContent`, `SetNullContent`, `StringContent`, `ToolCallId`) were verified verbatim against `dto/openai_request.go` / `dto/openai_response.go` during plan research.
- Known risk flagged inline: Task 5's sub-context channel selection (must be verified against `controller/relay.go` before wiring) and the SSE chunk shape (must match `stream-utils.ts`). Both have explicit verification steps.
