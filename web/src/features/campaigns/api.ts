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
import { api } from '@/lib/api'

import {
  campaignParticipantsDataSchema,
  campaignRewardsDataSchema,
  campaignStatsSchema,
  campaignsListDataSchema,
  type CampaignParticipantsData,
  type CampaignRewardsData,
  type CampaignStats,
  type CampaignsListData,
} from './types'

interface ApiEnvelope<T> {
  success: boolean
  message: string
  data?: T
  // Set when the operation succeeded but a side effect (e.g. invitation code
  // generation) failed; show as a warning, not an error.
  warning?: string
}

export async function getCampaigns(params: {
  p: number
  page_size: number
}): Promise<ApiEnvelope<CampaignsListData>> {
  const res = await api.get('/api/campaign/', { params })
  return {
    ...res.data,
    data: campaignsListDataSchema.parse(
      res.data?.data ?? { items: [], total: 0 }
    ),
  }
}

export async function searchCampaigns(params: {
  p: number
  page_size: number
  keyword: string
}): Promise<ApiEnvelope<CampaignsListData>> {
  const res = await api.get('/api/campaign/search', { params })
  return {
    ...res.data,
    data: campaignsListDataSchema.parse(
      res.data?.data ?? { items: [], total: 0 }
    ),
  }
}

export async function createCampaign(
  payload: Record<string, unknown>
): Promise<ApiEnvelope<unknown>> {
  const res = await api.post('/api/campaign/', payload)
  return res.data
}

export async function updateCampaign(
  payload: Record<string, unknown>
): Promise<ApiEnvelope<unknown>> {
  const res = await api.put('/api/campaign/', payload)
  return res.data
}

export async function updateCampaignStatus(
  id: number,
  status: number
): Promise<ApiEnvelope<unknown>> {
  const res = await api.put(`/api/campaign/${id}/status`, { status })
  return res.data
}

export async function deleteCampaign(
  id: number
): Promise<ApiEnvelope<unknown>> {
  const res = await api.delete(`/api/campaign/${id}`)
  return res.data
}

export async function getCampaignStats(
  id: number
): Promise<ApiEnvelope<CampaignStats>> {
  const res = await api.get(`/api/campaign/${id}/stats`)
  return { ...res.data, data: campaignStatsSchema.parse(res.data?.data ?? {}) }
}

export async function getCampaignParticipants(
  id: number,
  params: { p: number; page_size: number }
): Promise<ApiEnvelope<CampaignParticipantsData>> {
  const res = await api.get(`/api/campaign/${id}/participants`, { params })
  return {
    ...res.data,
    data: campaignParticipantsDataSchema.parse(
      res.data?.data ?? { items: [], total: 0 }
    ),
  }
}

export async function getCampaignRewards(
  id: number,
  params: { p: number; page_size: number }
): Promise<ApiEnvelope<CampaignRewardsData>> {
  const res = await api.get(`/api/campaign/${id}/rewards`, { params })
  return {
    ...res.data,
    data: campaignRewardsDataSchema.parse(
      res.data?.data ?? { items: [], total: 0 }
    ),
  }
}

export async function resendCampaignRewardEmail(
  rewardId: number
): Promise<ApiEnvelope<unknown>> {
  const res = await api.post(`/api/campaign/rewards/${rewardId}/resend`)
  return res.data
}
