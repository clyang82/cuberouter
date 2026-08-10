# Campaign System Port — Design Spec

> Date: 2026-08-06
> Source: `/home/skeeey/workspace/searouter-isuanova` (`docs/campaign-system.md`)
> Approach: **Faithful adapted port** — mirror the source backend structure 1:1, adapted to this repo's conventions; rewrite the frontend in this repo's TanStack Router + TypeScript + Base UI stack.

## 1. Scope decisions (approved by user)

| Decision | Choice |
|---|---|
| `phone_filled` campaign type | **Port it**, and add a `Phone` field to the User model so its triggers exist |
| Ops (运营) access tier | **Skip entirely** — no ops role, no `/api/ops/*` routes, no ops frontend, no invitee-scoped model queries |
| Frontend | **Rewrite** in this repo's stack, modeled on `web/src/features/redemption-codes/` |

## 2. Feature summary

The Campaign System is a marketing/promotion engine layered on the existing Redemption (兑换码) subsystem. It automatically dispatches quota rewards (via redemption codes) when users perform qualifying actions.

- **Campaign** (`campaigns` table): configurable activity with type, lifecycle status, `start_at`/`end_at` window, and JSON `config_json` reward rules.
- **CampaignParticipant** (`campaign_participants`): audit row per trigger event.
- **CampaignReward** (`campaign_rewards`): links user → campaign → redemption, with dispatch status and email-delivery tracking.

### Statuses

| Campaign status | Value | Reward status | Value |
|---|---|---|---|
| Draft | 1 | Pending | 1 |
| Active | 2 | Dispatched | 2 |
| Paused | 3 | Failed | 3 |
| Ended | 4 | Cancelled | 4 |

### Campaign types

| Type | Trigger | Reward mechanism |
|---|---|---|
| `phone_filled` | User first fills in a phone number | Engine mints a brand-new redemption code directly for the user and emails it. `MaxRewardsPerUser=1` prevents clear/refill abuse. |
| `invitation` | New user registers (built-in or OAuth) with an inviter (`aff` code) | Engine pulls one pre-generated code **atomically** from the invitee's redemption pool, credits the new user's quota, emails the code. Only the campaign whose `invitee_user_id == inviterId` fires. |

### `CampaignConfig` fields

`quota`, `redemption_name`, `redemption_count`, `max_participants` (0 = unlimited), `max_rewards_per_user` (0 = unlimited), `expire_days` (0 = never); invitation-only: `invitee_user_id`, `invitee_username`, `code_count` (capped at 1000, generated incrementally).

## 3. Backend design

### 3.1 `model/campaign.go` (new)

Constants: `CampaignStatus{Draft,Active,Paused,Ended}` = 1..4; `CampaignType{PhoneFilled,Invitation}`; `CampaignRewardStatus{Pending,Dispatched,Failed,Cancelled}` = 1..4.

Structs: `Campaign` (with `gorm.DeletedAt` soft delete), `CampaignConfig`, `CampaignParticipant` (+`ParticipantExtra`), `CampaignReward`.

Functions (ported verbatim unless noted):

- Campaign CRUD: `GetAllCampaigns`, `SearchCampaigns`, `GetCampaignById`, `GetActiveCampaignsByType` (status=Active AND `start_at <= now` AND (`end_at = 0` OR `end_at > now`)), `Insert`, `Update`, `Delete`, `DeleteCampaignById`.
- Participants: `CreateCampaignParticipant`, `CountCampaignParticipants`, `CountCampaignParticipantsByUser`, `GetCampaignParticipants`.
- Rewards: `CreateCampaignReward`, `CountCampaignRewards`, `CountDispatchedRewards`, `SumDispatchedQuota`, `GetCampaignRewards`, `HasPendingReward`, `GetCampaignRewardById`, `MarkRewardEmailSent` (uses `map[string]any` so the zero-value `EmailError=""` clear is not skipped), `MarkRewardEmailFailed` (truncates to 200 chars; leaves `EmailSentAt` untouched).
- Stats: `CampaignStats`, `GetCampaignStats`; `RecordCampaignLog(f)`; `ParseCampaignConfig`; unexported `campaignLog`.
- **Dropped**: `GetCampaignsByInviteeUserId`, `SearchCampaignsByInviteeUserId` (ops-only).

All JSON via `common.Marshal`/`common.Unmarshal`.

### 3.2 `model/redemption.go` (extend)

- Add field `OwnerAdminId int \`gorm:"type:int;default:0;index"\`` (identifies which invitee's pool a code belongs to).
- Add `CountAvailableRedemptionCodesByOwner(ownerAdminId int) (int64, error)` — enabled + non-expired codes for an owner.
- Add `DispatchRedemptionToUser(ownerAdminId, userId int) (*Redemption, error)` — **rewritten** to this repo's concurrency idiom (mirroring `Redeem`): transaction + `lockForUpdate(tx)` to select the oldest available code, then a compare-and-swap `status: enabled → used` update (guards SQLite where no row lock exists), then `quota += redemption.Quota` on the user. Returns `(nil, nil)` when the pool is empty. **The source's `tx.Set("gorm:query_option", "FOR UPDATE")` is not ported** — AGENTS.md forbids it (GORM v2 silently ignores it).

### 3.3 `model/user.go` (extend)

Add `Phone string \`json:"phone" gorm:"type:varchar(32);index"\`` to `User`. Surface in the request DTOs / update paths for: registration, admin create-user, admin update-user, self update-profile.

### 3.4 `model/main.go`

Register `&Campaign{}`, `&CampaignParticipant{}`, `&CampaignReward{}` in the existing `AutoMigrate` list. `OwnerAdminId` and `Phone` columns are added by AutoMigrate on all three supported databases (no hand-written DDL needed).

### 3.5 `service/campaign.go` (new)

- `CampaignEngine` struct + `CampaignEngineInstance` singleton.
- `OnPhoneFilled(user)` / `OnInvitationRegister(user, inviterId)`: nil/zero guards, then `gopool.Go(...)` → `handleCampaignType` → `GetActiveCampaignsByType` → per-campaign `processCampaignForUser`.
- `processCampaignForUser`: parse config; enforce `MaxParticipants`; enforce `MaxRewardsPerUser`; create `CampaignParticipant` (with inviter extra JSON when present); dispatch by type.
- `dispatchPhoneFilledReward`: quota guard; mint `Redemption` (UUID key, `Status=Enabled`, `ExpiredTime` from `ExpireDays`); on insert error record `Failed` reward; on success record `Dispatched` reward + `RecordLog(topup)` + async `SendCampaignRewardEmail`.
- `dispatchInvitationReward`: require `config.InviteeUserId != 0` and `== inviterId`; `DispatchRedemptionToUser` (nil → log + return, no reward row); error → record `Failed`; success → record `Dispatched` with `redemption.Quota`, `RecordLog`, async email.
- `GenerateInvitationCodes(campaign) (int, error)`: validate invitee exists and `quota > 0`; `codeCount` clamped to [1, 1000]; incremental — only generates `codeCount - available` deficit; key = `username-` + `UUID[:8]` with prefix truncated so key ≤ 32 chars; `OwnerAdminId` set; logs via `RecordCampaignLogf`.
- `SendCampaignRewardEmail(rewardId, campaign, redemption, user) error`: no user email → nil (silent skip); bilingual zh/en HTML body; SMTP failure → `MarkRewardEmailFailed` + return err; success → `MarkRewardEmailSent` (clears `EmailError`).
- The source's `common/email_template.go` helpers (`WrapBilingualSubject`, `WrapBilingualContent`) become **unexported functions in `service/campaign.go`** — this is their only caller (per AGENTS.md, no package-level single-use helpers).

### 3.6 `controller/campaign.go` (new) + `router/api-router.go`

Admin routes under `/api/campaign` with `AdminAuth()`:

```
GET    /                     GetAllCampaigns
GET    /search               SearchCampaigns
GET    /:id                  GetCampaign
POST   /                     AddCampaign
PUT    /                     UpdateCampaign
PUT    /:id/status           UpdateCampaignStatus
DELETE /:id                  DeleteCampaign
GET    /:id/stats            GetCampaignStats
GET    /:id/participants     GetCampaignParticipants
GET    /:id/rewards          GetCampaignRewards
POST   /rewards/:id/resend   ResendCampaignRewardEmail
```

Validation (ported from source): `AddCampaign` rejects empty name/type and unknown types; `invitation` requires `invitee_user_id` of an existing user and `quota > 0`; forces `RedemptionCount=1`; defaults status to Draft; calls `GenerateInvitationCodes` only when created non-Draft. `UpdateCampaignStatus` rejects invalid status values and triggers `GenerateInvitationCodes` on activation of an invitation campaign. `ResendCampaignRewardEmail` rejects non-Dispatched rewards, `RedemptionId==0`, and users without email. Response shape follows this repo's existing controller idiom (`{success, message, data}`).

**No ops routes.**

### 3.7 Trigger call sites

| File | Site | Trigger |
|---|---|---|
| `controller/user.go` | Register, phone provided | `OnPhoneFilled` |
| `controller/user.go` | Register, inviter resolved | `OnInvitationRegister` |
| `controller/user.go` | Admin update-user, phone empty→non-empty | `OnPhoneFilled` |
| `controller/user.go` | Self update-profile, phone first filled | `OnPhoneFilled` (async) |
| `controller/user.go` | Admin create-user with phone | `OnPhoneFilled` |
| `controller/oauth.go` | OAuth registration with inviter | `OnInvitationRegister` |

## 4. Frontend design

New module **`web/src/features/campaigns/`** modeled on `features/redemption-codes/` (`api.ts`, `constants.ts`, `types.ts`, `index.tsx`, `components/`, hooks), with route **`web/src/routes/_authenticated/campaigns/index.tsx`** (zod search schema, same as redemption-codes route) and a sidebar entry in the admin section.

Components:

- **Campaigns table**: list/search (by id or name prefix) with pagination, type/status badges, row actions (edit, status transitions, delete with confirm, view detail).
- **Create/edit dialog**: name, description, type, status, start/end datetime, config fields (quota, redemption_name, redemption_count, max_participants, max_rewards_per_user, expire_days); invitation-only fields (invitee user picker, code_count) shown conditionally.
- **Detail view**: stats cards (participants, rewards, dispatched, total quota), participants table, rewards table with per-row **resend email** action and email status (`email_sent_at`/`email_error`) display.

Phone field surfacing (needed to make `phone_filled` reachable from UI):

- Optional phone input on the **registration form**.
- Phone input in **profile settings** (self-update).
- Phone field in the **admin user create/edit** dialog.

**i18n**: all user-facing strings via `useTranslation()`/`t('...')`; keys added to every locale file under `web/src/i18n/locales/` using the repo's i18n tooling.

**No ops pages.**

## 5. Testing design

Backend tests use the repo's existing fixtures (sqlite in-memory DB, `truncateTables`, `testify/require`+`assert`). Coverage corresponds to the source doc's UT list, minus ops items (2, 3, 19):

- `model/campaign_test.go`
  - `GetActiveCampaignsByType`: only Active + within `[start_at, end_at)`; `end_at=0` = never expires; excludes draft/paused/ended and out-of-window.
  - `HasPendingReward` / `CountCampaignRewards` / `CountDispatchedRewards` / `SumDispatchedQuota` per campaign/status.
  - `MarkRewardEmailSent`: sets `email_sent_at` **and clears** a non-empty `email_error` (zero-value-skip regression).
  - `MarkRewardEmailFailed`: truncates to 200 chars; leaves `email_sent_at` untouched.
  - `ParseCampaignConfig`: empty → zero struct, no error; malformed → error.
- `model/redemption_test.go` (additions)
  - `DispatchRedemptionToUser` concurrency: N goroutines over a pool of M < N codes → exactly M successful dispatches and M quota increments, no double-issue.
  - `CountAvailableRedemptionCodesByOwner`: counts only enabled + non-expired.
- `service/campaign_test.go`
  - `processCampaignForUser`: skips on `MaxParticipants` reached; skips on `MaxRewardsPerUser` reached; records participant otherwise.
  - `dispatchPhoneFilledReward`: creates enabled Redemption + Dispatched reward with correct quota; insert failure → Failed reward with `RedemptionId=0`; `ExpireDays` sets/clears expiry.
  - `dispatchInvitationReward`: invitee mismatch → no-op; empty pool → no reward row, no error; dispatch failure → Failed; success → Dispatched with `redemption.Quota`.
  - `GenerateInvitationCodes`: incremental deficit only; cap 1000; skip when sufficient; requires `quota > 0` and existing invitee; prefix truncation keeps key ≤ 32 chars.
  - `OnPhoneFilled`/`OnInvitationRegister` guards: nil user / `Id==0` / `inviterId==0` are no-ops.
  - `SendCampaignRewardEmail`: no email → nil; SMTP failure → `EmailError` written + error returned; success → `EmailSentAt` written, `EmailError` cleared.
  - End-to-end invitation: active invitation campaign (codes generated) → register user with inviter → exactly one code consumed, one Dispatched reward, quota increased, participant recorded.
- `controller/campaign_test.go`
  - `AddCampaign` validation matrix (empty name/type, invalid type, invitation without invitee, invitation quota ≤ 0, forces `RedemptionCount=1`, Draft default, codes generated only when non-Draft).
  - `UpdateCampaignStatus`: invalid status rejected; activating an invitation campaign generates codes.
  - `ResendCampaignRewardEmail`: rejects non-Dispatched / `RedemptionId==0` / user without email.

Frontend tests (vitest, matching existing style): `features/campaigns` API layer and constants.

## 6. Docs

Port `docs/campaign-system.md` into this repo, adapted: ops tier removed, phone-field addition noted, trigger file:line references re-pointed, test section updated to reflect the tests now existing.

## 7. Out of scope

- Ops (运营) role, middleware, routes, pages, invitee-scoped queries.
- `ops/user` invitee history endpoints (already excluded with the ops tier).
- Any change to billing/quota math beyond the campaign flows described.
