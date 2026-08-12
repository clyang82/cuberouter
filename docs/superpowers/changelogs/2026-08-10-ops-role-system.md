# Changelog

All notable changes to this project are documented here, newest first.

## Unreleased

### Added

#### Ops Role System (v1)

An operations (运营) role ported from searouter-isuanova, sitting between common users and admins in the role hierarchy. It grants scoped, **read-only** access to two resource domains tied to the ops user's own identity: their invitees (users who registered under their `aff` invitation code) and the invitation campaigns where they are the linked invitee.

**Backend**

- `RoleOpsUser = 5` in `common/constants.go`; `IsValidateRole` accepts exactly the five roles (guest / common / ops / admin / root).
- `OpsAuth` middleware (`middleware/auth.go`) enforcing `role >= 5` through the shared `authHelper` — ops (5), admin (10), and root (100) all pass; common (1) and guest (0) are rejected. Disabled users are rejected regardless of role.
- `common.MaskPhone` phone-masking helper (`common/mask.go`).
- Inviter-scoped invitee queries in `model/user.go`, every one filtered `WHERE inviter_id = ?` with `Omit("password")`: `GetOpsInvitees` (paged, ordered `created_at DESC, id DESC`), `SearchOpsInvitees` (numeric keyword matches `id` exact plus `username`/`email`/`display_name`/`phone` LIKE; non-numeric matches the four LIKE fields only), `ExportOpsInviteesByIds` / `ExportOpsInviteesByKeyword` (batched at `opsExportBatchSize = 200`; ids belonging to other inviters are filtered out).
- Invitee-scoped campaign queries in `model/campaign.go`: `GetCampaignsByInviteeUserId` / `SearchCampaignsByInviteeUserId` use a `config_json` LIKE pre-filter plus a precise Go-side parse of `invitee_user_id`, so LIKE false positives (e.g. `:7` matching `:77`) are excluded.
- Ten read-only ops handlers: `controller/user_ops.go` (invitee list / search / CSV export), `controller/ops_user_columns.go` (`GetOpsUserColumns` column metadata), `controller/campaign_ops.go` (campaign detail / stats / participants / rewards). List and search responses mask phones; the CSV export is streamed with a UTF-8 BOM and excludes the phone column; exports are logged.
- `ManageUser` `promote_ops` / `demote_ops` actions (`controller/user.go`), admin/root only, behind the `canManageTargetRole` guard; error messages are i18n keys (`ops.*`), not hardcoded strings.
- Routes: `/api/ops/campaign` (6 routes) and `/api/ops/user` (4 routes) registered behind `OpsAuth` in `router/api-router.go`.
- Backend i18n keys `ops.*` in en, zh-CN, and zh-TW locales (`i18n/locales/*.yaml`).

**Frontend**

- `ROLE.OPS = 5` and its label key in `web/src/lib/roles.ts`; `isOps` / `isAdmin` role guards in `web/src/lib/role-guards.ts`.
- Sidebar ops section (`web/src/hooks/use-sidebar-data.ts`): Ops Campaigns → `/ops/campaign`, Invite History → `/ops/invite-history`, both gated at `requiredRole: ROLE.OPS` and narrowed by `filterNavGroupsByRole` (`nav-role-filter.ts`).
- `web/src/features/ops-campaigns/` — read-only campaign list plus a detail drawer with stats, participants, and rewards.
- `web/src/features/ops-users/` — invitee table with search, column selector backed by `GET /api/ops/user/columns`, and CSV export download.
- Routes `/ops/campaign` and `/ops/invite-history` redirect to `/403` in `beforeLoad` when the user is not ops.
- Admin Users page (`web/src/features/users/`): "Promote to Ops" / "Demote from Ops" row actions.
- i18n strings in en, zh, and zh-TW locales.

**Docs**

- Design spec: `docs/superpowers/specs/2026-08-10-ops-role-system-design.md`; operator guide: `docs/ops-role-system.md`.

**Tests**

- `common/mask_test.go` — `MaskPhone` edge cases (empty, very short, short, standard-length numbers).
- `middleware/auth_test.go` — OpsAuth role-threshold matrix (ops/admin/root allowed, common/guest rejected) and access-token fallback still enforcing the min role.
- `model/user_ops_test.go` — invitee scoping to `inviter_id`, pagination and ordering, keyword behavior, export id filtering and 200-batch behavior.
- `controller/user_ops_test.go` — phone masking, search keyword passthrough, CSV output (UTF-8 BOM + headers, ids-over-keyword precedence, unsupported-format rejection), column metadata.
- `controller/campaign_ops_test.go` — invitee-scoped campaign detail/stats/participants/rewards and exclusion of LIKE false positives.
- `controller/user_manage_test.go` — promote/demote ops permission matrix (admin/root succeed, common user rejected, already-ops / not-ops cases) and role persistence.
- Frontend: `web/src/lib/roles.test.ts`, `web/src/lib/role-guards.test.ts`, `web/src/components/layout/lib/__tests__/nav-role-filter.test.ts`.

**Known limitations**

- None.
