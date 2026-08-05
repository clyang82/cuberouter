# Plugin Market — v1 Design

**Date:** 2026-07-27
**Status:** Approved

## Goal

Admins register "plugins" — a bundle of (a) a remote MCP server and (b) a skill prompt loaded from a GitHub repo. Logged-in users invoke plugins in the built-in Playground chat by typing `@slug` in a message. The gateway injects the skill into the system prompt, exposes the MCP server's tools to the model, and runs the tool-call loop server-side. The user sees a spinner while tools run, then the final answer.

## Non-goals (v1)

- Stdio/local MCP servers (only remote HTTP/SSE).
- MCP server authentication (public servers only).
- Per-user or per-group plugin gating (all enabled plugins visible to all logged-in users).
- Surfacing tool-call steps in the UI (spinner only; details go to admin logs).
- A public "market" directory — "market" here means the admin-managed registry, not a discovery UI.
- Plugin invocation outside the Playground (`/v1/chat/completions` etc. are untouched).

## Prior art / constraints

- Playground endpoint `POST /pg/chat/completions` in `controller/playground.go` already builds relay info with the user's session/group and funnels into the standard `Relay` pipeline — so billing, channel selection, and logging come for free.
- No MCP client exists in the codebase; `pkg/mcp/` is greenfield.
- Tool/function-calling fields already exist on `dto/openai_request.go` (`Tools []ToolCallRequest`, `ToolChoice`) and pass through to upstream providers — we only need to *inject* and *intercept*, not extend the wire format.
- Frontend playground lives at `web/src/features/playground/`; stream parsing in `lib/streaming/stream-utils.ts` handles `reasoning`/`content` only. We do not add new SSE event types in v1.
- i18n: all new UI text via `t('English key')`, English source keys, sync with `bun run i18n:sync`.

## Data model

New table `plugins` via GORM (works on SQLite / MySQL / PostgreSQL — text columns only, no JSON columns, no dialect-specific features):

```go
// model/plugin.go
type Plugin struct {
    Id             int    `gorm:"primaryKey"`
    Name           string // display name
    Slug           string `gorm:"uniqueIndex;size:64"` // @slug in chat
    Description    string `gorm:"size:512"`
    Enabled        bool
    McpUrl         string `gorm:"size:1024"` // remote MCP endpoint
    SkillSource    string `gorm:"size:1024"` // GitHub URL
    SkillContent   string `gorm:"type:text"` // cached skill markdown
    SkillFetchedAt int64
    CreatedTime    int64
    UpdatedTime    int64
}
```

- Slug rules: `^[a-z0-9][a-z0-9-]{1,63}$`, unique, immutable after creation (rename = new plugin).
- In-memory cache `slug → *Plugin` refreshed on every CRUD, mirroring the pattern used by channel ability caches. No Redis dependency — playground is single-node for v1 semantics; DB is the source of truth.

## Skill loading (GitHub, server-side)

- On create/update/refresh, the backend resolves the GitHub URL to a raw file and fetches it:
  - Accept `https://github.com/{owner}/{repo}` (default branch, look for `SKILL.md` at repo root),
  - `.../tree/{branch}/{path}` (fetch `{path}/SKILL.md`),
  - or a direct `raw.githubusercontent.com` URL.
- Fetch with 10s timeout, 256 KiB cap, store snapshot in `SkillContent`, stamp `SkillFetchedAt`.
- Fetch failure does **not** block plugin creation — plugin is saved with empty `SkillContent` and an admin-visible warning on the list page. A plugin with no skill content just skips prompt injection.
- `POST /api/plugin/:id/refresh` re-fetches on demand.

## MCP client (`pkg/mcp/`)

Minimal JSON-RPC 2.0 client for remote MCP servers:

```go
type Client interface {
    ListTools(ctx context.Context) ([]Tool, error)
    CallTool(ctx context.Context, name string, arguments json.RawMessage) (Result, error)
}
```

- Transport: streamable HTTP (POST JSON-RPC, accept `application/json` and `text/event-stream` responses). No SSE GET long-poll in v1.
- Session: `initialize` handshake per tool-list/tool-call batch; `Mcp-Session-Id` header honored when returned.
- Timeouts: 5s connect, 10s `ListTools`, 30s `CallTool`.
- Tool list cached per plugin for 60s to avoid a `tools/list` round-trip on every chat message.
- No auth headers in v1.

## Backend API

Admin (behind `AdminAuth`, registered in `router/api-router.go`):

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/plugin/` | list all |
| POST | `/api/plugin/` | create |
| PUT | `/api/plugin/` | update |
| DELETE | `/api/plugin/:id` | delete |
| POST | `/api/plugin/:id/refresh` | re-fetch skill |
| POST | `/api/plugin/test` | body: `{mcp_url}`; ping server, return tool list — used by the admin form's "Test" button |

User (behind `UserAuth`):

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/plugin/enabled` | `[{slug, name, description}]` for `@` autocomplete |

## Playground agentic loop (core flow)

In `controller.Playground`, after request parsing and before `Relay`:

1. **Detect** — scan the *last user message* for `@slug` tokens with regex `@([a-z0-9][a-z0-9-]{1,63})` (word-boundary). Dedupe, resolve against enabled plugins, cap at 3 plugins per message. Unknown slugs are left as literal text — no error.
2. **Inject skill** — for each matched plugin with non-empty `SkillContent`, prepend to the system prompt:
   ```
   <plugin name="{slug}">
   {skill markdown}
   </plugin>
   ```
   If no system message exists, create one.
3. **Inject tools** — `ListTools` per plugin, convert to OpenAI tool format, rename to `{slug}__{tool_name}` to prevent cross-plugin collisions. Set on the request's `Tools` field; `ToolChoice` left to the model (`auto`).
4. **Loop** — run up to 10 rounds of **non-streaming internal relay calls**:
   - Invoke the internal relay with `stream=false`, full message history + injected tools.
   - If the response has no `tool_calls` → this is the final answer; break.
   - Otherwise, for each tool call: strip the `{slug}__` prefix, `CallTool` on the matching plugin's MCP client, append a `tool` role message with the result (or `"error: {msg}"` on failure — let the model recover, don't fail the request).
   - Loop cap reached → append system note `"tool round limit reached"` and use the last model response.
5. **Respond** — stream the final answer to the browser as normal `content` chunks. From the browser's perspective, the request simply has a long time-to-first-token while tools run.

**Billing:** every internal round goes through the standard `Relay` pipeline, so quota deduction, channel retry, and logging behave exactly as for N separate requests. No special billing logic, no new quota paths — the billing safety invariants in AGENTS.md are untouched.

**Failure modes:**
- MCP server unreachable at inject time → skip that plugin's tools, still inject its skill, note in log `other`.
- Tool call errors → fed back to the model as tool-message content.
- All internal rounds respect the user's existing quota; insufficient quota mid-loop returns whatever error the relay produced, formatted as the final message.

**Admin observability:** each tool round's request/response rides the existing log pipeline; we add `plugin_slugs` and tool-call counts into the consume log's `other` field for debugging. No new log types.

## Frontend

**Admin plugin management** — new `web/src/features/plugins/` + sidebar entry (respecting `use-sidebar-config` conventions):
- List page: name, slug, MCP URL, skill source, enabled toggle, last-fetched time, refresh button, edit/delete.
- Edit drawer: fields per the model; "Test connection" button hitting `/api/plugin/test` and showing the returned tool list; "Fetch skill now" preview.
- All text i18n'd; follow existing `features/channels/` drawer/dialog patterns.

**Playground** (`web/src/features/playground/`):
- Input component: typing `@` opens an autocomplete fed by `GET /api/plugin/enabled` (cached for the session). Selecting inserts `@slug ` as plain text.
- Send flow unchanged — slugs stay in the message text; the backend parses them.
- While waiting for first token on a message that mentions `@slug`, the pending-message spinner shows an extended label: `t('Using {{slug}}…')`. If no plugin is mentioned, the existing spinner is unchanged.
- No new SSE event types; `stream-utils.ts` untouched.

## Error handling summary

| Case | Behavior |
|---|---|
| Unknown `@slug` | treated as plain text |
| Skill fetch fails at save | plugin saved without skill; admin warning |
| MCP unreachable at chat time | tools skipped for that plugin, skill still injected, logged |
| Tool call fails | error string sent back as tool message; model continues |
| Loop cap (10) hit | return last model response with system note |
| Quota exhausted mid-loop | relay error surfaced as the final assistant message |

## Testing

Backend (Go, `testify/require` + `assert`, table-driven where it fits):
- `model/plugin.go` — slug validation, CRUD, cache refresh.
- Skill fetch — GitHub URL resolution matrix (root / tree path / raw URL), size cap, failure modes (use `httptest.Server`, not the real GitHub).
- `pkg/mcp/` — JSON-RPC handshake, `ListTools`/`CallTool` against an in-process fake MCP server (`httptest`).
- Playground loop — slug detection regex (edge cases: email addresses, `@@`, punctuation boundaries), prompt/tool injection shape, loop termination (no tools, max rounds, tool error recovery), namespacing collisions.
- Cross-DB: model uses only portable column types; no migration logic beyond `AutoMigrate`.

Frontend: covered by existing build/type checks; no new test infra in v1.

## Security notes

- MCP URL is admin-controlled; still, the MCP client enforces timeouts and response size caps (1 MiB) to bound damage from a hostile server.
- Tool results are inserted as `tool` role messages, never as system — prompt-injection surface is limited to model-visible content, same as any upstream tool response.
- `/api/plugin/enabled` exposes only slug/name/description — never MCP URLs or skill content.
