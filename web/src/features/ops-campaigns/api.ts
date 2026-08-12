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

import type {
  Campaign,
  CampaignParticipantsData,
  CampaignRewardsData,
  CampaignStats,
  CampaignsListData,
} from './types'

interface ApiEnvelope<T> {
  success: boolean
  message: string
  data?: T
}

export async function getOpsCampaigns(params: {
  p: number
  page_size: number
}): Promise<ApiEnvelope<CampaignsListData>> {
  const res = await api.get('/api/ops/campaign/', { params })
  return res.data
}

export async function searchOpsCampaigns(params: {
  keyword: string
  p: number
  page_size: number
}): Promise<ApiEnvelope<CampaignsListData>> {
  const res = await api.get('/api/ops/campaign/search', { params })
  return res.data
}

export async function getOpsCampaign(
  id: number
): Promise<ApiEnvelope<Campaign>> {
  const res = await api.get(`/api/ops/campaign/${id}`)
  return res.data
}

export async function getOpsCampaignStats(
  id: number
): Promise<ApiEnvelope<CampaignStats>> {
  const res = await api.get(`/api/ops/campaign/${id}/stats`)
  return res.data
}

export async function getOpsCampaignParticipants(
  id: number,
  params: { p: number; page_size: number }
): Promise<ApiEnvelope<CampaignParticipantsData>> {
  const res = await api.get(`/api/ops/campaign/${id}/participants`, { params })
  return res.data
}

export async function getOpsCampaignRewards(
  id: number,
  params: { p: number; page_size: number }
): Promise<ApiEnvelope<CampaignRewardsData>> {
  const res = await api.get(`/api/ops/campaign/${id}/rewards`, { params })
  return res.data
}
