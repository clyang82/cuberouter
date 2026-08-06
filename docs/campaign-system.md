# Campaign System (活动/营销系统)

> Last Updated: 2026-08-06

## 1. Feature Description

The Campaign System is a marketing/promotion engine that automatically dispatches **quota rewards (via redemption codes 兑换码)** to users when they perform qualifying actions. It is layered on top of the existing Redemption (兑换码) subsystem.

### Core concepts

- **Campaign** (`campaigns` table) — a configurable marketing activity with a type, lifecycle status, time window (`start_at`/`end_at`), and a JSON `config_json` holding reward rules.
- **CampaignParticipant** (`campaign_participants`) — an audit record of each trigger/participation event (one per user per qualifying action).
- **CampaignReward** (`campaign_rewards`) — a reward record linking a user → campaign → redemption code, with dispatch status and email-delivery tracking.

### Lifecycle & reward statuses

| Campaign status | Value | Reward status | Value |
|-----------------|-------|---------------|-------|
| Draft (草稿) | 1 | Pending (待发放) | 1 |
| Active (进行中) | 2 | Dispatched (已发放) | 2 |
| Paused (已暂停) | 3 | Failed (发放失败) | 3 |
| Ended (已结束) | 4 | Cancelled (已取消) | 4 |

### Campaign types

| Type | Trigger | Reward mechanism |
|------|---------|-------------------|
| `phone_filled` | User first fills in a phone number (register, admin-create, admin-update, self-update) | Engine mints a brand-new redemption code directly for the user and emails it. `MaxRewardsPerUser=1` prevents phone clear/refill abuse. |
| `invitation` | A new user registers (built-in or OAuth) with an inviter (`aff` code) | Engine pulls one pre-generated code **atomically** from the inviter's redemption-code pool (`owner_admin_id = invitee_user_id`), credits the new user's quota, and emails the code. Only the campaign whose `invitee_user_id == inviterId` fires. |

### Per-campaign config (`CampaignConfig`)

`quota` (reward amount), `redemption_name`, `redemption_count` (forced to `1` by `AddCampaign`), `max_participants` (0 = unlimited), `max_rewards_per_user` (0 = unlimited), `expire_days` (0 = never expires), plus invitation-only `invitee_user_id`, `invitee_username`, `code_count` (batch size, capped at 1000, generated incrementally).

### Access tier

- **Admin** (`/api/campaign`, `AdminAuth`) — full CRUD, stats, participants, rewards, and resend-reward-email.

### Reward delivery

On successful dispatch, a bilingual (zh/en) HTML email is sent with the redemption code; `CampaignReward.EmailSentAt` / `EmailError` track delivery state. Admins can re-trigger via `POST /api/campaign/rewards/:id/resend`.

---

## 2. Related Code and Code Logic

### Backend files

| File | Role |
|------|------|
| `model/campaign.go` | Data models, constants (`CampaignStatusDraft/Active/Paused/Ended` = 1..4, `CampaignTypePhoneFilled`/`CampaignTypeInvitation`), `ParseCampaignConfig`, paginated getters returning `(total, items, err)` |
| `model/redemption.go` | `OwnerAdminId` pool field, `CountAvailableRedemptionCodesByOwner`, `DispatchRedemptionToUser` — the pool the invitation type draws from |
| `model/user.go` | `Phone varchar(32);index` on `User`; `EditWithTx` (map-based, can clear phone), `UpdateWithTx` (struct-based, cannot clear — zero-skip) |
| `service/campaign.go` | `CampaignEngine` — trigger handling, sync guards (pre-gopool), reward dispatch, `GenerateInvitationCodes`, bilingual email helpers (`wrapBilingualSubject`/`wrapBilingualContent`), `SendCampaignRewardEmail` |
| `controller/campaign.go` | Admin REST handlers (CRUD, stats, participants, rewards, resend) |
| `router/api-router.go` (lines 271–285) | Route registration under `/api/campaign` with `AdminAuth` (11 routes) |
| `model/main.go` (lines 306, 371) | Auto-migration of `Campaign`, `CampaignParticipant`, `CampaignReward` |
| `controller/user.go`, `controller/oauth.go` | Trigger call sites |

### Data models (`model/campaign.go`)

- `Campaign` — `Id, Name, Description, Type, Status, StartAt, EndAt, ConfigJson, CreatedBy, CreatedAt, UpdatedAt, DeletedAt` (soft delete via `gorm.DeletedAt`).
- `CampaignConfig` (JSON-in-text) — `Quota, RedemptionName, RedemptionCount, MaxParticipants, MaxRewardsPerUser, ExpireDays`, plus invitation-only `InviteeUserId, InviteeUsername, CodeCount`.
- `CampaignParticipant` — `CampaignId, UserId, EventType, EventAt, ExtraJson` (holds `InviterId/InviterName`).
- `CampaignReward` — `CampaignId, UserId, RedemptionId, Quota, Status, DispatchedAt, CreatedAt, EmailSentAt, EmailError`.

### Core logic flow (`service/campaign.go`)

```
Trigger (user.go / oauth.go)
  └─ CampaignEngineInstance.OnPhoneFilled(user)      // or OnInvitationRegister(user, inviterId)
       └─ sync guards first (nil user / Id==0 / inviterId==0 short-circuit), then gopool.Go(...)  // async, non-blocking
            └─ handleCampaignType(type, user, inviterId)
                 └─ GetActiveCampaignsByType(type)   // status=Active AND within start/end window
                      └─ for each campaign: processCampaignForUser(...)
                           ├─ ParseCampaignConfig()
                           ├─ enforce MaxParticipants  (global cap)
                           ├─ enforce MaxRewardsPerUser (per-user cap)
                           ├─ CreateCampaignParticipant(...)   // audit row
                           └─ switch campaign.Type:
                                ├─ phone_filled → dispatchPhoneFilledReward
                                │     ├─ create Redemption (status=Enabled) directly for user
                                │     ├─ recordReward(..., Dispatched) or ...Failed on error
                                │     ├─ RecordLog(topup)
                                │     └─ gopool.Go(SendCampaignRewardEmail)
                                └─ invitation → dispatchInvitationReward
                                      ├─ guard: config.InviteeUserId == inviterId
                                      ├─ DispatchRedemptionToUser(inviteeId, userId)
                                      │     (tx: lockForUpdate → CAS on RowsAffected → quota += → mark used)
                                      ├─ recordReward(..., Dispatched) or ...Failed
                                      ├─ RecordLog(topup)
                                      └─ gopool.Go(SendCampaignRewardEmail)
```

### Key design points

- Guards (nil user, `Id==0`, `inviterId==0`) run **synchronously before** `gopool.Go`, so invalid triggers never enter the pool; valid triggers run **asynchronously** so registration/login is never blocked.
- `GetActiveCampaignsByType` filters by `status = Active` AND `start_at <= now AND (end_at = 0 OR end_at > now)` — the time window is enforced at query time.
- Invitation dispatch is **atomic**: `DispatchRedemptionToUser` (`model/redemption.go`) runs in a transaction using the shared `lockForUpdate(tx)` helper (`FOR UPDATE` on MySQL/PostgreSQL, skipped on SQLite) plus a compare-and-swap update guarded by `RowsAffected`; concurrent dispatchers cannot double-issue a code. Returns `(nil, nil)` when the owner's pool is empty — callers treat empty pool as a no-op, not an error.
- `GenerateInvitationCodes` is **incremental**: it counts already-available codes owned by the invitee (`CountAvailableRedemptionCodesByOwner`) and only generates the deficit up to `code_count` (capped at 1000). Code keys are `prefix` (username truncated to ≤ 24 chars) + `UUID[:8]`, guaranteed to fit `char(32)`. Requires `quota > 0` and `InviteeUserId != 0`. Called on campaign create (if active), update (if active), and status-change-to-active.
- `AddCampaign` **forces `RedemptionCount = 1`** — each reward is exactly one redemption code worth `quota`.
- `MarkRewardEmailSent` / `MarkRewardEmailFailed` use `map[string]any` updates so GORM doesn't skip zero-value clears (e.g. clearing `EmailError` on a successful resend); failure messages are truncated to 200 chars.
- Bilingual email helpers `wrapBilingualSubject` / `wrapBilingualContent` produce zh+en subject/body for reward delivery.

### Trigger call sites

| File:line | Context |
|-----------|---------|
| `controller/user.go:293` | `Register` — registration with phone filled → `OnPhoneFilled` |
| `controller/user.go:296` | `Register` — registration with inviter → `OnInvitationRegister` |
| `controller/user.go:752` | `UpdateUser` — admin updates user, phone goes empty→non-empty → `OnPhoneFilled` |
| `controller/user.go:948` | `UpdateSelf` (non-password branch) — self-update, phone first filled → `OnPhoneFilled` |
| `controller/user.go:1081` | `CreateUser` — admin creates user with phone → `OnPhoneFilled` |
| `controller/oauth.go:435` | OAuth registration with inviter → `OnInvitationRegister` |

Admin edits go through `model.User.EditWithTx` (map-based updates — can both set and clear `phone`). Self-service edits go through `model.User.UpdateWithTx` (struct-based `Updates` — skips zero values, so it can set a phone but **cannot clear** one; see Known Limitations).

### REST API

**Admin** (`/api/campaign`, `AdminAuth`): `GET /`, `GET /search`, `GET /:id`, `POST /`, `PUT /`, `PUT /:id/status`, `DELETE /:id`, `GET /:id/stats`, `GET /:id/participants`, `GET /:id/rewards`, `POST /rewards/:id/resend`.

### Frontend

React 19 + Rsbuild feature module, modeled on the redemption-codes feature:

- `web/src/features/campaigns/` — feature root (`api.ts`, `types.ts`, `constants.ts`, `index.tsx`), with `components/` (table, columns, row actions, primary buttons, provider, dialogs, detail drawer, mutate drawer, delete dialog) and `lib/campaign-form.ts` (form ⇄ `config_json` mapping, datetime-local ⇄ unix-seconds conversion).
- Route: `web/src/routes/_authenticated/campaigns/index.tsx`; sidebar entry registered alongside redemption codes.
- Phone inputs added to: sign-up form, profile (phone-bind dialog), and the admin users drawer.

---

## 3. Tests

The Campaign System is covered by seven test files (six Go, one frontend). Source test cases 2, 3, and 19 (Ops/运营 tier scoping) were **dropped** because the Ops access tier was not ported, per spec. All other source cases are implemented; mapping below.

### model layer

`model/campaign_test.go`

- `TestGetActiveCampaignsByType` — source case 1: only `Active` campaigns within `[start_at, end_at)`; `end_at=0` = never-expire; excludes draft/paused/ended and out-of-window.
- `TestSearchCampaigns` — admin search by id/name prefix.
- `TestCampaignRewardCounts` / `TestCampaignParticipantCounts` — source case 4: reward and participant counts/sums per campaign/status.
- `TestMarkRewardEmailSentClearsError` — source case 5: sets `email_sent_at` **and** clears a non-empty `email_error` (zero-value-skip regression).
- `TestMarkRewardEmailFailed` — source case 6: truncates `errMsg` to 200 chars; leaves `email_sent_at` untouched.
- `TestParseCampaignConfig` — source case 7: empty `ConfigJson` → zero-value struct; malformed JSON → error.
- `TestGetCampaignStats` — aggregated stats query.

`model/redemption_dispatch_test.go`

- `TestCountAvailableRedemptionCodesByOwner` — counts only unused, not-expired codes for an owner.
- `TestDispatchRedemptionToUser_EmptyPool` — returns `(nil, nil)` (no error) when the owner pool is empty.
- `TestDispatchRedemptionToUser_Success` — credits user quota, marks code used, all inside one transaction.
- `TestDispatchRedemptionToUser_ConcurrentNoDoubleIssue` — source case 14: N concurrent dispatchers over M < N codes yield exactly M successes and M quota increments (no double-issue).

`model/user_phone_test.go`

- `TestUserPhonePersistedViaEditWithTx` — admin edit path persists (and can clear) `phone`.
- `TestUserPhoneUpdateWithTxKeepsNonEmpty` — `UpdateWithTx` struct-update keeps a non-empty phone (verifies zero-skip semantics).

### service layer (`service/campaign_test.go`)

- `TestCampaignEngineGuards` / `TestOnPhoneFilledDispatchesAsynchronously` — source case 12: no-op on nil user / `Id==0` / `inviterId==0`; dispatch is asynchronous and does not block the caller.
- `TestProcessCampaignForUserCaps` — source case 8: skips on `MaxParticipants` / `MaxRewardsPerUser`; records a participant otherwise.
- `TestProcessCampaignForUserRecordsInviter` — inviter id/name recorded in participant `ExtraJson`.
- `TestDispatchPhoneFilledReward` — source case 9: mints Enabled redemption + Dispatched reward with correct quota; insert failure → Failed reward with `RedemptionId=0`; respects `ExpireDays`.
- `TestDispatchInvitationReward` — source case 10: `InviteeUserId != inviterId` is a no-op; empty pool → no reward, no error; failure → Failed; success → Dispatched with `redemption.Quota`.
- `TestGenerateInvitationCodes` / `TestGenerateInvitationCodesCap` — source case 11: incremental deficit generation, 1000 cap, `quota > 0` and `InviteeUserId != 0` enforcement, key-length ≤ 32.
- `TestSendCampaignRewardEmail` — source case 13: no email → nil; SMTP failure → `EmailError` written + error returned; success → `EmailSentAt` set, `EmailError` cleared.

### controller layer

`controller/campaign_test.go`

- `TestAddCampaign` — source case 16: rejects empty name/type and invalid type; `invitation` requires `invitee_user_id` (existing user) and `quota > 0`; forces `RedemptionCount=1`; defaults to Draft; generates codes only if not Draft.
- `TestUpdateCampaignStatus` — source case 17: rejects invalid status; activating an invitation campaign triggers `GenerateInvitationCodes`.
- `TestResendCampaignRewardEmail` — source case 18: rejects non-Dispatched rewards / `RedemptionId==0` / user with no email; surfaces `SendCampaignRewardEmail` errors.

`controller/campaign_trigger_test.go`

- `TestRegisterTriggersCampaigns` — source case 15 (end-to-end): register with phone + inviter fires both triggers; codes consumed, rewards dispatched, quota credited, participants recorded.

### frontend (`web/src/features/campaigns/lib/__tests__/campaign-form.test.ts`, 10 tests)

Form ⇄ `config_json` mapping: zero ↔ empty-string conversion, datetime-local ↔ unix-seconds round-trip, defaults for empty/invalid JSON, full-config parse, `redemption_count=1` force for invitation, invitee-field zeroing for `phone_filled`, create defaults, and id handling on create vs update.

### Dropped source cases

| Source case | Reason |
|-------------|--------|
| 2 (`GetCampaignsByInviteeUserId`) | Ops/运营 tier not ported |
| 3 (`SearchCampaignsByInviteeUserId`) | Ops/运营 tier not ported |
| 19 (Ops scoping, "无权查看") | Ops/运营 tier not ported |

---

## 4. Known Limitations

- **Phone unbind is admin-only by design.** The self-service phone-bind dialog offers bind/change only: it rejects empty submissions (prohibiting an implied unbind) and states that removing a bound phone requires an administrator. On the backend this matches reality — the dialog submits through `UpdateSelf` (`controller/user.go`), which persists via `model.User.UpdateWithTx` (struct-based GORM `Updates` that skip zero-values, so an empty phone string would be a no-op anyway). Clearing works via the **admin edit path** (`UpdateUser` → `EditWithTx`, which uses a `map[string]any` update and therefore honors empty strings). The asymmetry is intentional (it also means a user cannot strip their phone to re-farm a `phone_filled` campaign reward) and is covered by `TestUserPhoneUpdateWithTxKeepsNonEmpty` / `TestUserPhonePersistedViaEditWithTx`.

