# Changelog

All notable changes to this project are documented here, newest first.

## Unreleased

### Added

#### Campaign System (v1)

A campaign/activity system ported from searouter-isuanova: admins create reward campaigns that grant redemption-code quota to users when a trigger event fires, with a full admin management UI.

**Backend**

- New `campaigns` and `campaign_participants` tables (`model/campaign.go`) with GORM models working on SQLite, MySQL, and PostgreSQL.
- Two campaign types: `phone_filled` (user binds a phone number) and `invitation` (a registered user is claimed by an invitee's campaign).
- Admin CRUD + status endpoints under `/api/campaign/` (`controller/campaign.go`, `router/api-router.go`); campaign types are immutable after creation.
- Trigger hooks: phone binding (`controller/user.go`) and OAuth/registration flows (`controller/oauth.go`) evaluate active campaigns and dispatch rewards.
- Reward dispatch (`service/campaign.go`): each qualifying event records a participant, draws a code from the campaign's redemption pool, credits the quota to the user, and sends a reward email. Failures to generate the code pool are surfaced to the admin via a `warning` field on add/update/status responses.
- Invitation campaigns maintain a per-campaign redemption code pool (`redemptions.campaign_id`, `model/redemption.go`) topped up to the configured `code_count` whenever the campaign is activated; pools are scoped by (campaign, invitee) so campaigns never share or suppress each other's codes.
- Atomic participant admission: max-participant and per-user reward-limit checks run in a transaction holding the campaign row lock (`lockForUpdate`), so concurrent triggers cannot overshoot limits; the participant slot is released if the pool is empty so a later top-up can still reach that user.
- Dispatch is transactional: quota credit requires the recipient row to exist (`RowsAffected == 1`), otherwise everything rolls back.
- Invitation reward email is a credit receipt (codes are auto-redeemed at dispatch) rather than redemption instructions; content locked in by a contract test.
- `UpdateCampaign` merges partial requests via a pointer-field DTO: omitted fields keep stored values, explicit zeroes still clear.
- Phone binding: users can bind a phone number from profile settings (`phone-bind-dialog`), and admins can set phone numbers in the user edit drawer.

**Frontend**

- New Campaigns admin page (`web/src/features/campaigns/`): data table with status filters, create/edit drawer, detail drawer showing participants and dispatched rewards (with reward-email resend), delete dialog, and row-level quick status actions (activate/pause/end).
- Sidebar entry wired in `use-sidebar-data`; i18n strings added to en, zh, and zh-TW locales.
- Campaign form schema lives in `features/campaigns/lib/campaign-form.ts` (zod, `z.infer` form type) with unit tests; all mutations go through `useMutation` + `handleServerError`.
- Sign-up form carries the inviter's campaign code so invitation rewards attribute correctly.

**Docs**

- Design spec: `docs/superpowers/specs/2026-08-06-campaign-system-design.md`; operator guide: `docs/campaign-system.md`.

**Known issues**

- Redemption codes generated before the `campaign_id` column existed have `campaign_id = 0` and are outside every campaign pool; re-activating an invitation campaign tops its pool back up to `code_count`.
