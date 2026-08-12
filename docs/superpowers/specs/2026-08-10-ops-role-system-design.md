# Ops Role System Port — Design Spec

> Date: 2026-08-10
> Source: `/home/skeeey/workspace/searouter-isuanova` (`docs/ops-role-system.md`)
> Approach: **Faithful adapted port** — mirror the source backend structure 1:1, adapted to this repo's conventions; rewrite the frontend in this repo's TanStack Router + TypeScript + Base UI stack; add the unit tests the source never had.

## 1. Scope decisions

The previous campaign-system port (2026-08-06) deliberately **skipped** the ops tier ("no ops role, no `/api/ops/*` routes, no ops frontend, no invitee-scoped model queries"). This spec completes that deferred tier.

| Decision | Choice |
|---|---|
| Backend | **Full port**: role constant, `OpsAuth`, `MaskPhone`, ops invitee model queries, invitee-scoped campaign queries (re-adding the two dropped functions), three ops controller files, `promote_ops`/`demote_ops` in `ManageUser`, routes, i18n keys |
| Frontend | **Full port, rewritten** in this repo's stack: two ops pages, ops sidebar section, `promote_ops`/`demote_ops` actions in the admin Users page |
| Tests | **Add all applicable cases from source doc §3** (cases 1–19 backend; frontend cases 20–21 adapted) — the source has zero ops tests |
| Error messages | Source hardcodes Chinese strings; this repo is i18n-driven → new backend i18n keys in `en.yaml`/`zh-CN.yaml`/`zh-TW.yaml` |
| `New-Api-User` header (source auth) | **N/A** — this repo's `authenticateDashboardRequest` has no such anti-CSRF header check; source doc test case 3 does not apply |
| i18n locales | `en` / `zh` / `zh-TW` only (matches the campaign port and the existing locale files) |

## 2. Feature summary

The Ops role (运营) sits between common users and admins. It grants **read-only** access to two resource domains tied to the ops user's own identity:

- **Invitee management** — view/search/export the users they invited (users with `inviter_id == self`), phone numbers masked.
- **Invitation campaigns** — view (stats/participants/rewards) invitation campaigns where `config_json.invitee_user_id == self`.

Role hierarchy: Guest 0 / Common 1 / **Ops 5** / Admin 10 / Root 100. `OpsAuth` uses `minRole = 5`, so admin/root also pass. Only admin/root can grant or revoke the ops role.

## 3. Backend design

### 3.1 `common/constants.go`

Add `RoleOpsUser = 5` between `RoleCommonUser` and `RoleAdminUser`; extend `IsValidateRole` to accept it. No schema change (role column is a plain int).

### 3.2 `common/mask.go` (new)

Port `MaskPhone` verbatim (preserve first 3 / last 4 with 4 stars; short numbers 3–7 → first + stars + last; ≤2 → all stars; empty → empty). Source has no tests for it — add `common/mask_test.go` with a deterministic table test.

### 3.3 `middleware/auth.go`

Add `OpsAuth()` returning `authHelper(c, common.RoleOpsUser)`, next to `AdminAuth`/`RootAuth` (which already take the `minRole` threshold). No other auth changes: session/PAT handling, disabled-user rejection, and role-threshold enforcement all come from the existing `authHelper`.

### 3.4 `model/user.go` (extend)

Port the four ops queries + `opsExportBatchSize = 200`:

- `GetOpsInvitees(inviterId, pageInfo)` — count + `Omit("password")`, `Where("inviter_id = ?")`, `Order("created_at DESC, id DESC")`, offset/limit.
- `SearchOpsInvitees(inviterId, keyword, startIdx, num)` — same scoping; numeric keyword also matches `id = ?`; always scoped to caller.
- `ExportOpsInviteesByIds(inviterId, ids)` — batched at 200, `id IN ? AND inviter_id = ?` per batch (foreign ids filtered out), errors logged via `common.SysLog`, batch skipped.
- `ExportOpsInviteesByKeyword(inviterId, keyword)` — page through `SearchOpsInvitees` until a short page; `common.SysLog` on error.

`GetUserById`, `Phone`, `InviterId`, `AffCode`, `AffCount` all already exist in this repo's `User` model.

### 3.5 `model/campaign.go` (extend)

Re-add the two functions the campaign port dropped:

- `GetCampaignsByInviteeUserId(userId, startIdx, num)` — LIKE pre-filter `config_json LIKE '%"invitee_user_id":<id>%'` (DB-side), then Go-side precise filter via `ParseCampaignConfig() == userId` (LIKE false-positives like `:123` matching `:12` are filtered out). Paging applied after the precise filter. `id desc` order.
- `SearchCampaignsByInviteeUserId(userId, keyword, startIdx, num)` — same, plus numeric keyword → `id = ? OR name LIKE ?`, else `name LIKE ?` (prefix match, `keyword%`).

### 3.6 `controller/user_ops.go` (new)

- `opsUserExportHeaders` (zh CSV columns: ID/用户名/显示名/状态/分组/总额度/已用额度/请求次数/创建时间/邀请码/邀请数/邀请人 ID).
- `formatOpsUserRow(u *User) []string` — CSV row; the status→zh mapping from the source's `statusZh` is **inlined** here (this repo has no `statusZh`; per AGENTS.md a single-use package helper should not be extracted — the mapping is two cases).
- `GetOpsInvitees` / `SearchOpsInvitees` — `c.GetInt("id")` scoping, `common.GetPageQuery`, `Omit` handled in model, `MaskPhone` on every row, `pageInfo.SetTotal/SetItems` + `common.ApiSuccess`.
- `ExportOpsInvitees` — `ExportOpsInviteesRequest{Ids, Keyword, Format}`; reject non-`csv` format; stream CSV with UTF-8 BOM + `Content-Disposition` filename `邀请用户_<ts>.csv`; ids take precedence over keyword; `common.SysLog` the export (userId, username, count).

### 3.7 `controller/ops_user_columns.go` (new)

`OpsUserColumnMeta{Key, Label, Required}` + static `opsUserColumns` list (id/username required) + `GetOpsUserColumns` returning `{success, message, data}`.

### 3.8 `controller/campaign_ops.go` (new)

Six read-only handlers, all `c.GetInt("id")`-scoped:

- `GetOpsCampaigns` / `SearchOpsCampaigns` — delegate to the §3.5 model functions.
- `GetOpsCampaign` / `GetOpsCampaignStats` / `GetOpsCampaignParticipants` / `GetOpsCampaignRewards` — load campaign, parse config, reject with the not-accessible message when `config.InviteeUserId != userId`; participants/rewards join `Username` via `GetUserById(p.UserId, false)` (missing user → empty username, as in source).

**i18n**: the four source messages ("无权查看该活动"/"…统计"/"…参与记录"/"…奖励记录") become one key with a subject argument: `ops.campaign_not_accessible` = `无权查看该活动{{.Subject}}` (subject = "", "统计", "参与记录", "奖励记录"). "无效的活动 ID" → `ops.invalid_campaign_id`. Used via `common.ApiErrorI18n`.

### 3.9 `controller/user.go` — `ManageUser` ops actions

Add two cases to the existing switch (which already runs `canManageTargetRole(myRole, targetRole)` first — root bypass + `myRole > targetRole`, equivalent to the source guard, so non-root admins still can't act on peers):

- `promote_ops` — reject when `myRole < RoleAdminUser` (`i18n.MsgUserAdminCannotPromote`); reject when `user.Role >= RoleOpsUser` (new key `ops.user_already_ops_or_higher`); else `user.Role = RoleOpsUser`.
- `demote_ops` — reject when `myRole < RoleAdminUser`; reject when `user.Role != RoleOpsUser` (new key `ops.user_not_ops`); else `user.Role = RoleCommonUser`.

Both fall through the **existing common tail** — `user.Update(false)` (which publishes the auth version and revokes browser sessions exactly once, per the existing `demote` precedent), token-cache invalidation, `recordManageAuditFor(c, user.Id, "user.manage", {action, username, id})`, and the `{Role, Status}` response. No authz-casbin cleanup (ops role is outside the authz system; only `demote` from admin touches it).

### 3.10 `router/api-router.go`

Register right after the existing `/api/campaign` group (line ~284), mirroring the source layout:

```
opsCampaignRoute := apiRouter.Group("/ops/campaign").Use(middleware.OpsAuth())
  GET /               GetOpsCampaigns
  GET /search         SearchOpsCampaigns
  GET /:id            GetOpsCampaign
  GET /:id/stats      GetOpsCampaignStats
  GET /:id/participants  GetOpsCampaignParticipants
  GET /:id/rewards    GetOpsCampaignRewards

opsUserRoute := apiRouter.Group("/ops/user").Use(middleware.OpsAuth())
  GET /columns        GetOpsUserColumns
  GET /               GetOpsInvitees
  GET /search         SearchOpsInvitees
  POST /export        ExportOpsInvitees
```

### 3.11 Backend i18n (`i18n/keys.go` + `locales/{en,zh-CN,zh-TW}.yaml`)

New keys: `ops.campaign_not_accessible` (with `{{.Subject}}`), `ops.invalid_campaign_id`, `ops.user_already_ops_or_higher`, `ops.user_not_ops`, `ops.export_unsupported_format` (with `{{.Format}}`). Keys follow the existing `noun.verb` naming and `{{.X}}` arg style.

## 4. Frontend design

Rewritten in this repo's stack, modeled on `web/src/features/campaigns/` (list + detail) and `web/src/features/users/` (row actions, API client). All user-facing strings via `t()` into `en.json`/`zh.json`/`zh-TW.json`.

### 4.1 `web/src/lib/roles.ts`

Add `OPS: 5` to `ROLE`, add `[ROLE.OPS]: 'Ops'` to `ROLE_LABEL_KEYS`, add the `'Ops'` key to all three locale files. Add a small pure `web/src/lib/role-guards.ts` with `isOps(role)` / `isAdmin(role)` (role-threshold helpers, adaptation of the source's `utils.jsx isOps`) so route guards and sidebar gating share tested logic.

### 4.2 Sidebar (`web/src/hooks/use-sidebar-data.ts`, `use-sidebar-view.ts`)

- `use-sidebar-data.ts`: new `ops` group after `admin` with items `Ops Campaigns` (`/ops/campaign`, `Megaphone` icon) and `Invite History` (`/ops/invite-history`, `Users` icon), each with `requiredRole: ROLE.OPS`.
- `use-sidebar-view.ts`: extend the group filter — `group.id === 'admin' ? isAdmin : group.id === 'ops' ? isOps : true`. `requiredRole` item filtering already exists.
- No `sidebar_modules` config changes needed: `URL_TO_CONFIG_MAP` has no entry for the ops URLs, so they default to visible ("No mapping config, default to visible"), and role gating happens via `requiredRole`.
- Role changes take effect on next auth refresh (this repo's `auth-store` re-fetches `/api/user/self` at boot; no localStorage caching, unlike the source).

### 4.3 Routes

`web/src/routes/_authenticated/ops/campaign/index.tsx` and `web/src/routes/_authenticated/ops/invite-history/index.tsx`, both with `beforeLoad` gating like `campaigns/index.tsx`: `if (!auth.user || auth.user.role < ROLE.OPS) throw redirect({to: '/403'})`, plus zod `validateSearch` for `{page, pageSize, keyword}`.

### 4.4 `web/src/features/ops-campaigns/` (new)

`api.ts` (typed envelopes + zod schemas, `api.get('/api/ops/campaign/…')`), `types.ts` (reuse campaign types where possible), `constants.ts`, `index.tsx`, components: data table (name/id/status/type/period + stats columns), search box, detail drawer with three sections — stats, participants, rewards — read-only, no action buttons.

### 4.5 `web/src/features/ops-users/` (new)

`api.ts` (`/api/ops/user/` list, `/search`, `/columns`, `/export` as blob download), `types.ts` (invitee row), `index.tsx`: data table (columns from `/columns` metadata — id/username required, others toggleable), keyword search, export button (POST `{keyword}` or `{ids}` → `text/csv` blob download), phone shown masked.

### 4.6 `web/src/features/users/` (extend)

- `types.ts`: extend `ManageUserAction` union with `'promote_ops' | 'demote_ops'`.
- `lib/user-actions.ts`: success messages for the two actions.
- `components/data-table-row-actions.tsx`: add dropdown items — `Promote to Ops` (shown when `user.role < 5`, not deleted) and `Demote from Ops` (when `user.role === 5`), both via the existing `handleManage('promote_ops'|'demote_ops')` → `manageUser` API; error/success toasts use the same patterns as promote/demote.
- Role cell label automatically picks up the new 'Ops' label via `getRoleLabel`.

### 4.7 Frontend i18n

`en.json` / `zh.json` / `zh-TW.json`: 'Ops', 'Ops Campaigns', 'Invite History', 'Promote to Ops', 'Demote from Ops', table labels (invitee columns), export button, empty states, plus action success/error messages.

## 5. Testing

All new backend tests follow repo style: `require` for setup/fatal, `assert` for value checks, deterministic table tests, explicit fixture state (sqlite in-memory `gorm.Open` per test file, same as `model/campaign_test.go` / `controller/campaign_test.go` / `middleware/auth_test.go`).

### 5.1 `middleware/auth_test.go` (extend)

1. `OpsAuth` accepts roles 5 / 10 / 100; rejects 1 and 0 with 403 (source doc case 1; case 3 — `New-Api-User` header — N/A here).
2. `OpsAuth` rejects a disabled user even with a valid role (case 2).
3. Access-token (PAT) auth fallback still enforces `minRole` for ops routes (case 4, adapted).

### 5.2 `model/user_ops_test.go` (new)

4. `GetOpsInvitees` — only `inviter_id == caller`; other ops users' invitees never returned; `password` never present in result rows; pagination respected (page_size/offset); order `created_at DESC, id DESC` (case 5).
5. `SearchOpsInvitees` — numeric keyword matches `id` exact + four LIKE fields; non-numeric matches only LIKE fields; always scoped (case 6).
6. `ExportOpsInviteesByIds` — foreign ids filtered out; batches at 200; no password (case 7).
7. `ExportOpsInviteesByKeyword` — paginates until short page; aggregates; scoped (case 8).

### 5.3 `model/campaign_test.go` (extend)

8. `GetCampaignsByInviteeUserId` / `SearchCampaignsByInviteeUserId` — return only campaigns whose parsed `invitee_user_id == userId`; LIKE false-positives (e.g. `:12` vs `:123`, `:1234`) excluded (source doc case 14, model level).

### 5.4 `controller/user_ops_test.go` (new)

9. `GetOpsInvitees` masks phones; pagination passes through `PageInfo` (case 9).
10. `SearchOpsInvitees` masks phones; `keyword` query param; empty keyword behaves like list (case 10).
11. `ExportOpsInvitees` — UTF-8 BOM + header row; honors `ids` when non-empty else `keyword`; rejects unsupported `format` (case 11).
12. `GetOpsUserColumns` — static metadata with `required` on id/username (case 12).

### 5.5 `controller/campaign_ops_test.go` (new)

13. `GetOpsCampaign` / `GetOpsCampaignStats` / `GetOpsCampaignParticipants` / `GetOpsCampaignRewards` — "无权查看" when `invitee_user_id != current user`; succeed when it matches (case 13).
14. `GetOpsCampaigns` / `SearchOpsCampaigns` — only campaigns with parsed `invitee_user_id == current user` (case 14, controller level).

### 5.6 `controller/user_manage_test.go` (extend)

15. `promote_ops` — admin (10) succeeds on common (1→5); root succeeds; common (1) rejected with `MsgUserAdminCannotPromote`; already-ops (5) rejected with `ops.user_already_ops_or_higher`; admin (10) rejected (case 15).
16. `demote_ops` — admin/root succeed (5→1); non-ops rejected with `ops.user_not_ops`; admin target rejected; `canManageTargetRole` blocks non-root acting on a peer (case 16).
17. After promote/demote the persisted `user.Role` equals 5 / 1 (case 17).

### 5.7 `common/mask_test.go` (new)

`MaskPhone` table test: 3–7-char and long forms, ≤2, empty.

### 5.8 Frontend (adapted cases 20–21)

18. `web/src/lib/role-guards.ts` unit tests — `isOps`/`isAdmin` mirror the role thresholds (case 20, adapted; route `beforeLoad` uses them).
19. `roles.ts` — `ROLE.OPS === 5` and label key present.
20. Sidebar ops-section visibility test where feasible (case 21, adapted): ops items appear only when role ≥ 5 (if the `useSidebarView` filtering logic can be tested without a full hook harness, extract the pure filter predicate into `use-sidebar-view`-adjacent pure helper; otherwise covered by 4.2 code review + route-level tests).

## 6. Docs

- Changelog: `docs/superpowers/changelogs/2026-08-10-ops-role-system.md` (Added section, mirroring the campaign changelog structure).
- Operator guide: `docs/ops-role-system.md` (adapted from the source doc, trimmed to what's relevant in this repo — e.g. no `New-Api-User` header mention, i18n keys instead of hardcoded strings).
