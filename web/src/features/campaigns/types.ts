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
import { z } from 'zod'

export const campaignTypeSchema = z.enum(['phone_filled', 'invitation'])
export type CampaignType = z.infer<typeof campaignTypeSchema>

export const campaignConfigSchema = z.object({
  quota: z.number().int().min(0).catch(0),
  redemption_name: z.string().catch(''),
  redemption_count: z.number().int().min(0).catch(0),
  max_participants: z.number().int().min(0).catch(0),
  max_rewards_per_user: z.number().int().min(0).catch(0),
  expire_days: z.number().int().min(0).catch(0),
  invitee_user_id: z.number().int().min(0).catch(0),
  invitee_username: z.string().catch(''),
  code_count: z.number().int().min(0).catch(0),
})
export type CampaignConfig = z.infer<typeof campaignConfigSchema>

export const campaignSchema = z.object({
  id: z.number(),
  name: z.string(),
  description: z.string(),
  type: z.string(),
  status: z.number(),
  start_at: z.number(),
  end_at: z.number(),
  config_json: z.string(),
  created_by: z.number(),
  created_at: z.number(),
  updated_at: z.number(),
})
export type Campaign = z.infer<typeof campaignSchema>

export const campaignParticipantSchema = z.object({
  id: z.number(),
  campaign_id: z.number(),
  user_id: z.number(),
  event_type: z.string(),
  event_at: z.number(),
  extra_json: z.string(),
  username: z.string().optional(),
})
export type CampaignParticipant = z.infer<typeof campaignParticipantSchema>

export const campaignRewardSchema = z.object({
  id: z.number(),
  campaign_id: z.number(),
  user_id: z.number(),
  redemption_id: z.number(),
  quota: z.number(),
  status: z.number(),
  dispatched_at: z.number(),
  created_at: z.number(),
  email_sent_at: z.number(),
  email_error: z.string(),
  username: z.string().optional(),
})
export type CampaignReward = z.infer<typeof campaignRewardSchema>

export const campaignStatsSchema = z.object({
  participant_count: z.number(),
  reward_count: z.number(),
  dispatched_count: z.number(),
  total_quota: z.number(),
})
export type CampaignStats = z.infer<typeof campaignStatsSchema>

export const campaignsListDataSchema = z.object({
  items: z.array(campaignSchema),
  total: z.number(),
})
export const campaignParticipantsDataSchema = z.object({
  items: z.array(campaignParticipantSchema),
  total: z.number(),
})
export const campaignRewardsDataSchema = z.object({
  items: z.array(campaignRewardSchema),
  total: z.number(),
})
export type CampaignsListData = z.infer<typeof campaignsListDataSchema>
export type CampaignParticipantsData = z.infer<
  typeof campaignParticipantsDataSchema
>
export type CampaignRewardsData = z.infer<typeof campaignRewardsDataSchema>

export type CampaignsDialogType = 'create' | 'update' | 'delete' | 'detail'
