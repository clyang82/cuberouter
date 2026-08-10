/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { TFunction } from 'i18next'
import { z } from 'zod'

import { CAMPAIGN_CODE_COUNT_MAX, CAMPAIGN_TYPE } from '../constants'
import {
  campaignConfigSchema,
  campaignTypeSchema,
  type Campaign,
  type CampaignConfig,
} from '../types'

export function getCampaignFormSchema(t: TFunction) {
  return z
    .object({
      name: z.string().min(1, t('Campaign name is required')),
      description: z.string(),
      type: campaignTypeSchema,
      status: z.number().int().min(1).max(4),
      start_at: z.string(),
      end_at: z.string(),
      quota: z.coerce
        .number()
        .int(t('Quota must be an integer'))
        .min(0, t('Quota cannot be negative')),
      redemption_name: z.string(),
      expire_days: z.coerce
        .number()
        .int(t('Expire days must be an integer'))
        .min(0, t('Expire days cannot be negative')),
      max_participants: z.coerce
        .number()
        .int(t('Max participants must be an integer'))
        .min(0, t('Max participants cannot be negative')),
      max_rewards_per_user: z.coerce
        .number()
        .int(t('Max rewards per user must be an integer'))
        .min(0, t('Max rewards per user cannot be negative')),
      invitee_user_id: z.coerce
        .number()
        .int(t('Invitee user ID must be an integer'))
        .min(0, t('Invitee user ID cannot be negative')),
      invitee_username: z.string(),
      code_count: z.coerce
        .number()
        .int(t('Code count must be an integer'))
        .min(1, t('Code count must be at least 1'))
        .max(
          CAMPAIGN_CODE_COUNT_MAX,
          t('Code count cannot exceed {{max}}', {
            max: CAMPAIGN_CODE_COUNT_MAX,
          })
        ),
    })
    .superRefine((values, ctx) => {
      if (values.type !== CAMPAIGN_TYPE.INVITATION) return
      if (values.invitee_user_id <= 0) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['invitee_user_id'],
          message: t('Invitee user is required for invitation campaigns'),
        })
      }
      if (values.quota <= 0) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['quota'],
          message: t('Reward quota must be greater than 0'),
        })
      }
    })
}

export type CampaignFormData = z.infer<
  ReturnType<typeof getCampaignFormSchema>
>

export function toDatetimeLocalValue(ts: number): string {
  if (!ts) return ''
  const d = new Date(ts * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function fromDatetimeLocalValue(value: string): number {
  if (!value) return 0
  const ms = new Date(value).getTime()
  if (Number.isNaN(ms)) return 0
  return Math.floor(ms / 1000)
}

export function buildCampaignConfigJson(form: CampaignFormData): string {
  const isInvitation = form.type === CAMPAIGN_TYPE.INVITATION
  const config: CampaignConfig = {
    quota: form.quota,
    redemption_name: form.redemption_name,
    redemption_count: isInvitation ? 1 : 0, // backend force-sets 1 for invitation too; keep client consistent
    max_participants: form.max_participants,
    max_rewards_per_user: form.max_rewards_per_user,
    expire_days: form.expire_days,
    invitee_user_id: isInvitation ? form.invitee_user_id : 0,
    invitee_username: isInvitation ? form.invitee_username : '',
    code_count: isInvitation ? form.code_count : 0,
  }
  return JSON.stringify(config)
}

export function parseCampaignConfig(configJson: string): CampaignConfig {
  const fallback = campaignConfigSchema.parse({})
  if (!configJson) return fallback
  try {
    const parsed = campaignConfigSchema.safeParse(JSON.parse(configJson))
    return parsed.success ? parsed.data : fallback
  } catch {
    return fallback
  }
}

export function campaignToFormDefaults(
  campaign?: Campaign | null
): CampaignFormData {
  if (!campaign) {
    return {
      name: '',
      description: '',
      type: CAMPAIGN_TYPE.PHONE_FILLED,
      status: 1,
      start_at: '',
      end_at: '',
      quota: 0,
      redemption_name: '',
      expire_days: 0,
      max_participants: 0,
      max_rewards_per_user: 0,
      invitee_user_id: 0,
      invitee_username: '',
      code_count: 100,
    }
  }
  const config = parseCampaignConfig(campaign.config_json)
  return {
    name: campaign.name,
    description: campaign.description,
    type:
      campaign.type === CAMPAIGN_TYPE.INVITATION
        ? CAMPAIGN_TYPE.INVITATION
        : CAMPAIGN_TYPE.PHONE_FILLED,
    status: campaign.status,
    start_at: toDatetimeLocalValue(campaign.start_at),
    end_at: toDatetimeLocalValue(campaign.end_at),
    quota: config.quota,
    redemption_name: config.redemption_name,
    expire_days: config.expire_days,
    max_participants: config.max_participants,
    max_rewards_per_user: config.max_rewards_per_user,
    invitee_user_id: config.invitee_user_id,
    invitee_username: config.invitee_username,
    code_count: config.code_count > 0 ? config.code_count : 100,
  }
}

export function buildCampaignPayload(
  form: CampaignFormData,
  id?: number
): Record<string, unknown> {
  return {
    ...(id ? { id } : {}),
    name: form.name,
    description: form.description,
    type: form.type,
    status: form.status,
    start_at: fromDatetimeLocalValue(form.start_at),
    end_at: fromDatetimeLocalValue(form.end_at),
    config_json: buildCampaignConfigJson(form),
  }
}
