# Ops User Role System (运营角色系统)

> Last Updated: 2026-08-10

## 1. Feature Description

The Ops role is a dedicated operations/运营 role in the user hierarchy, sitting between common users and admins. It grants scoped, **read-only** access to two resource domains tied to the ops user's own identity:

- **Invitee management** — view/search/export the users they invited (via `aff` invitation codes).
- **Invitation campaigns** — view (stats/participants/rewards) the invitation campaigns where they are the linked invitee (`config_json.invitee_user_id == self`).

### Role hierarchy (`common/constants.go`)

| Constant | Value | Meaning |
|----------|-------|---------|
| `RoleGuestUser` | 0 | Guest |
| `RoleCommonUser` | 1 | Ordinary registered user |
| `RoleOpsUser` | 5 | Ops / 运营 |
| `RoleAdminUser` | 10 | Admin |
| `RoleRootUser` | 100 | Super admin |

`IsValidateRole` accepts exactly these five values. Auth middleware uses a `minRole` threshold, so `OpsAuth` (min = 5) is also satisfied by admin (10) and root (100).

### What an Ops user can do

- **Invitee list** (`GET /api/ops/user/`) — paginated list of users with `inviter_id == self`, phone numbers masked (`MaskPhone`).
- **Invitee search** (`GET /api/ops/user/search`) — keyword search (id / username / email / display_name / phone) within their own invitees.
- **Invitee export** (`POST /api/ops/user/export`) — CSV export by `ids` or `keyword`, streamed with UTF-8 BOM; batched in pages of 200. The CSV omits the phone column entirely, so phone numbers never leave the server in unmasked form.
- **Ops columns** (`GET /api/ops/user/columns`) — column metadata for the frontend table.
- **Campaign views** (`/api/ops/campaign/...`) — read-only campaign detail / stats / participants / rewards, scoped to campaigns whose `invitee_user_id == self`.

### What an Ops user cannot do

- Create / update / delete campaigns (admin-only routes under `/api/campaign`).
- Manage other users (no write access to users outside their own invitees; invitee endpoints are read-only).
- Access admin/root-only endpoints (admin panel, settings, channel, etc.) — `AdminAuth` (min = 10) rejects role 5.

### Permission flow

The role threshold is enforced at three layers:

1. **Backend middleware** — `OpsAuth()` (`middleware/auth.go`) runs `authHelper(c, common.RoleOpsUser)`: it authenticates the dashboard request (session, or an access token when there is no session), rejects disabled users, and enforces `role >= 5`. Unlike the searouter-isuanova source, this port does **not** require a `New-Api-User` header — this repo's dashboard auth handles session/CSRF validation itself.
2. **Frontend navigation** — the `ops` sidebar group (Ops Campaigns, Invite History) is shown only for `role >= 5` via `filterNavGroupsByRole` and `requiredRole: ROLE.OPS` (`web/src/hooks/use-sidebar-data.ts`, `web/src/components/layout/lib/nav-role-filter.ts`).
3. **Frontend routes** — `/ops/campaign` and `/ops/invite-history` redirect to `/403` in `beforeLoad` when the user is not ops (`web/src/routes/_authenticated/ops/...`).

Error messages in this port are **i18n keys** (`ops.*`), not hardcoded strings — translations live in `i18n/locales/{en,zh-CN,zh-TW}.yaml` (e.g. `ops.user_already_ops_or_higher` = "用户已经是运营或更高级别角色"). A route-level config like the source's `setting/admin_permission.yaml` is not used here; the sidebar permission model is role-threshold based.

### Data isolation

Every ops query is scoped to the caller's own identity:

- **Invitee queries** (`model/user.go`) filter `WHERE inviter_id = ?`, where the value is the ops user's own id taken from the authenticated session.
- **Campaign queries** (`model/campaign.go`) use a `config_json` LIKE pre-filter plus a precise Go-side parse of `invitee_user_id`; `loadOpsCampaign` verifies the same per-campaign before serving detail/stats/participants/rewards.

An ops user can never see another user's invitees or campaigns, even by guessing ids or keywords.

---

## 2. Related Code and Code Logic

### Backend files

| File | Role |
|------|------|
| `common/constants.go` | Role constants + `IsValidateRole` |
| `middleware/auth.go` | `OpsAuth()` middleware → `authHelper(c, common.RoleOpsUser)` |
| `common/mask.go` | `MaskPhone` phone-masking helper |
| `controller/user_ops.go` | Ops invitee controllers: list/search/export + CSV formatting |
| `controller/ops_user_columns.go` | `GetOpsUserColumns` — column metadata for the ops invitee table |
| `controller/campaign_ops.go` | Ops campaign controllers (read-only, invitee-scoped) |
| `controller/user.go` | `ManageUser` `promote_ops` / `demote_ops` actions |
| `model/user.go` | `GetOpsInvitees`, `SearchOpsInvitees`, `ExportOpsInviteesByIds`, `ExportOpsInviteesByKeyword` |
| `model/campaign.go` | `GetCampaignsByInviteeUserId`, `SearchCampaignsByInviteeUserId` |
| `router/api-router.go` | Route registration for `/api/ops/campaign` and `/api/ops/user` |

### Auth flow (`middleware/auth.go`)

`OpsAuth()` → `authHelper(c, common.RoleOpsUser)`:

1. Authenticates the dashboard request (session, or validates an access token when there is no session).
2. Rejects disabled users (`UserStatusDisabled`).
3. Enforces `role >= minRole` (i.e. `>= 5` for ops).
4. Sets `username/role/id/group` on the gin context for handlers.

The ops controllers then scope every query by `c.GetInt("id")` (the ops user's own id) — this is the key data-isolation mechanism.

### Ops invitee controllers (`controller/user_ops.go`)

- `GetOpsInvitees` → `model.GetOpsInvitees(userId, pageInfo)` — paged, `Omit("password")`, ordered `created_at DESC, id DESC`; masks `Phone` via `common.MaskPhone`.
- `SearchOpsInvitees` → `model.SearchOpsInvitees(userId, keyword, ...)` — same scoping + keyword on id/username/email/display_name/phone.
- `ExportOpsInvitees` — streams CSV; by `ids` (`ExportOpsInviteesByIds`, batched 200) or by `keyword` (`ExportOpsInviteesByKeyword`, paged 200). Writes UTF-8 BOM + `opsUserExportHeaders`, logs the export with userId/username/count. `ids` take precedence when both are provided; an unsupported `format` is rejected with `ops.export_unsupported_format`.
- `formatOpsUserRow` — maps a `User` to a CSV row (id, username, display_name, status, group, quota, used_quota, request_count, created_at, aff_code, aff_count, inviter_id). Note there is **no phone column** in the export.

### Ops invitee model (`model/user.go`)

All four functions filter `WHERE inviter_id = ?` (the ops user), so an ops user can never see another ops user's invitees. `Omit("password")` on every query. Export batches use `opsExportBatchSize = 200`; a failed batch is logged and skipped rather than failing the whole export.

### Ops campaign model & controllers (`model/campaign.go`, `controller/campaign_ops.go`)

`GetOpsCampaign(s)`, `GetOpsCampaignStats`, `GetOpsCampaignParticipants`, `GetOpsCampaignRewards` each first load the campaign, parse `config_json`, and reject with `ops.campaign_not_accessible` if `config.InviteeUserId != current user`. `GetOpsCampaigns`/`SearchOpsCampaigns` use `GetCampaignsByInviteeUserId` / `SearchCampaignsByInviteeUserId`: a `config_json LIKE '%"invitee_user_id":<id>%'` pre-filter narrows the rows, then Go-side parsing keeps only campaigns whose parsed `invitee_user_id` matches exactly (excludes LIKE false positives such as `:7` matching `:77`).

### Role promotion/demotion (`controller/user.go` ManageUser)

```
case "promote_ops":  // admin (10) or root (100) only
  if myRole < RoleAdminUser → MsgUserAdminCannotPromote
  if user.Role >= RoleOpsUser → ops.user_already_ops_or_higher
  user.Role = RoleOpsUser
case "demote_ops":   // admin or root only
  if myRole < RoleAdminUser → MsgUserAdminCannotPromote
  if user.Role != RoleOpsUser → ops.user_not_ops
  user.Role = RoleCommonUser
```

`ManageUser` also guards `canManageTargetRole(myRole, user.Role)` (root, or `myRole > targetRole`) before the switch, so a non-root actor cannot act on an equal/higher role. In the admin dashboard, use the Users page row actions **Promote to Ops** / **Demote from Ops** to grant or revoke the role.

### Routes (`router/api-router.go`)

```
// Campaign management (ops — read-only, filtered by invitee_user_id)
opsCampaignRoute := apiRouter.Group("/ops/campaign").Use(middleware.OpsAuth())
  GET /            GetOpsCampaigns
  GET /search      SearchOpsCampaigns
  GET /:id         GetOpsCampaign
  GET /:id/stats   GetOpsCampaignStats
  GET /:id/participants  GetOpsCampaignParticipants
  GET /:id/rewards       GetOpsCampaignRewards

// User invitee history (ops — read-only + export, filtered by inviter_id = current user)
opsUserRoute := apiRouter.Group("/ops/user").Use(middleware.OpsAuth())
  GET /columns     GetOpsUserColumns
  GET /            GetOpsInvitees
  GET /search      SearchOpsInvitees
  POST /export     ExportOpsInvitees
```

### Frontend

React 19 + Rsbuild feature modules on the repo's TanStack Router stack:

- `web/src/features/ops-campaigns/` — read-only campaign list (`ops-campaigns-table.tsx`) plus a detail drawer (`ops-campaigns-detail-drawer.tsx`) with stats, participants, and rewards tabs.
- `web/src/features/ops-users/` — invitee table with search, column selector (`ops-users-column-selector.tsx`, backed by `GET /api/ops/user/columns`), and CSV export download.
- Routes `web/src/routes/_authenticated/ops/campaign/index.tsx` and `web/src/routes/_authenticated/ops/invite-history/index.tsx` — `beforeLoad` role gate (`isOps`) redirecting to `/403`.
- Sidebar: `ops` group in `web/src/hooks/use-sidebar-data.ts` (Ops Campaigns with Megaphone icon, Invite History with Users icon), both `requiredRole: ROLE.OPS`; `filterNavGroupsByRole` (`web/src/components/layout/lib/nav-role-filter.ts`) narrows groups by role.
- Role helpers: `web/src/lib/roles.ts` (`ROLE.OPS = 5`, label key `Ops`), `web/src/lib/role-guards.ts` (`isOps` / `isAdmin`).
- Admin Users page (`web/src/features/users/`): "Promote to Ops" / "Demote from Ops" row actions calling `ManageUser`.
- i18n strings in en, zh, and zh-TW locales.

---

## 3. Tests

### Backend

- `common/mask_test.go` — `TestMaskPhone`: empty, very short (<= 2), short (3–7), and standard-length numbers.
- `middleware/auth_test.go` — `TestOpsAuthRoleThreshold`: ops/admin/root allowed, common/guest rejected; `TestOpsAuthSessionAccessTokenStillEnforcesMinRole`: access-token fallback still enforces the min role.
- `model/user_ops_test.go` — invitee scoping to `inviter_id` + ordering, pagination, keyword behavior (numeric id-exact vs. LIKE fields), export id filtering of foreign invitees and 200-batch behavior, keyword export pagination.
- `controller/user_ops_test.go` — phone masking in list/search, search keyword passthrough (empty keyword behaves like list), CSV output (UTF-8 BOM + headers, ids-over-keyword precedence, unsupported-format rejection), column metadata with `required` flags.
- `controller/campaign_ops_test.go` — invitee-scoped campaign detail/stats/participants/rewards (rejected for other invitees' campaigns) and exclusion of LIKE false positives in list/search.
- `controller/user_manage_test.go` — `TestManageUserPromoteOps` / `TestManageUserDemoteOps`: permission matrix (admin/root succeed, common user rejected, already-ops / not-ops cases) and persisted role after promote/demote.

### Frontend (`bun test`)

- `web/src/lib/roles.test.ts` — ops role constant is 5 and has a label key.
- `web/src/lib/role-guards.test.ts` — `isOps` accepts ops/admin/root, rejects common/guest; `isAdmin` accepts admin/root only.
- `web/src/components/layout/lib/__tests__/nav-role-filter.test.ts` — common user sees only the console group; ops user sees console + ops but not admin; admin sees all groups with `requiredRole` filtering; root sees everything; missing role behaves like guest.

---

## 4. Known Limitations

- **Search is LIKE-based** — invitee search matches partial substrings across username/email/display_name/phone; only the numeric `id` match is exact. The same applies to campaign search (`name LIKE keyword%`).
- **CSV export omits the phone column** — phones are masked in list/search responses and absent from the export entirely, by design.
- **Campaign scoping is computed in Go** — the `config_json` LIKE pre-filter is not an exact DB predicate; pagination and totals are computed after the Go-side parse, which is fine at realistic campaign volumes but should be revisited if a single user is ever linked to very large numbers of campaigns.
