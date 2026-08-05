# Changelog

All notable changes to this project are documented here, newest first.

## Unreleased

### Added

#### Plugin Market (v1)

Admins can register plugins — a bundle of a remote MCP server and a skill prompt fetched from GitHub — and logged-in users can invoke them in the built-in Playground chat by typing `@slug` in a message.

**Admin**

- New **Plugins** management page (sidebar → Admin): create/edit/delete plugins, enable/disable toggle, "Test connection" (lists the MCP server's tools), skill fetch status with on-demand refresh.
- New admin APIs: `GET/POST/PUT /api/plugin/`, `DELETE /api/plugin/:id`, `POST /api/plugin/:id/refresh`, `POST /api/plugin/test`.
- Plugin definition: name, unique immutable slug (`^[a-z0-9][a-z0-9-]{1,63}$`), description, MCP URL (remote HTTP/SSE, no auth), skill source (GitHub repo URL, `tree/{branch}/{path}` URL, or raw URL). The skill markdown is fetched server-side (10s timeout, 256 KiB cap) and cached in the database; a failed fetch never blocks saving — the admin sees a warning instead.

**Playground**

- Typing `@` in the chat input opens an autocomplete dropdown fed by `GET /api/plugin/enabled`; selecting inserts the slug as plain text.
- When a message mentions `@slug`, the backend injects the plugin's skill into the system prompt and exposes the MCP server's tools to the model (namespaced `{slug}__{tool}`, up to 3 plugins per message), then runs a server-side agentic tool-call loop (up to 10 rounds) through the standard relay pipeline — billing, channel selection, retries, and logging behave exactly as for normal requests.
- While tools run, the pending message shows a `Using {slug}…` spinner; the final answer streams as a normal chat response. Tool-call details stay in admin logs (consume log `other` carries `plugin_slugs` and `plugin_tool_calls` per round).

**Failure behavior**

- Unknown `@slug` → treated as plain text. MCP server unreachable at chat time → that plugin's tools are skipped, skill still injected. Tool call errors → fed back to the model as tool messages. Loop cap reached → a final tool-less round is forced with a `"tool round limit reached"` system note. Relay failure mid-loop → the relay error text is streamed as the final assistant message.

**Infrastructure**

- New `plugins` table (portable across SQLite/MySQL/PostgreSQL).
- New `pkg/mcp` package: minimal JSON-RPC 2.0 client for streamable-HTTP MCP servers (initialize handshake, `Mcp-Session-Id`, 60s tool-list cache, response size caps).
