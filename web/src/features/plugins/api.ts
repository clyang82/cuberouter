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

import type { EnabledPlugin, Plugin, PluginTestResult } from './types'

type ApiEnvelope<T> = {
  success: boolean
  message?: string
  data?: T
}

export async function getPlugins(): Promise<Plugin[]> {
  const res = await api.get('/api/plugin/')
  const body = res.data as ApiEnvelope<Plugin[]>
  return body?.data ?? []
}

export async function createPlugin(
  plugin: Partial<Plugin>
): Promise<ApiEnvelope<Plugin>> {
  const res = await api.post('/api/plugin/', plugin)
  return res.data
}

export async function updatePlugin(
  plugin: Partial<Plugin>
): Promise<ApiEnvelope<Plugin>> {
  const res = await api.put('/api/plugin/', plugin)
  return res.data
}

export async function deletePlugin(
  id: number
): Promise<ApiEnvelope<unknown>> {
  const res = await api.delete(`/api/plugin/${id}`, {
    skipBusinessError: true,
  })
  return res.data
}

export async function refreshPluginSkill(
  id: number
): Promise<ApiEnvelope<Plugin>> {
  const res = await api.post(
    `/api/plugin/${id}/refresh`,
    {},
    { skipBusinessError: true }
  )
  return res.data
}

export async function testPluginConnection(
  mcpUrl: string,
  authToken?: string,
  authHeader?: string
): Promise<PluginTestResult> {
  // Skip the global business-error toast so the caller can surface the
  // failure in the drawer's own context instead of showing "0 tools".
  const res = await api.post(
    '/api/plugin/test',
    {
      mcp_url: mcpUrl,
      auth_token: authToken || '',
      auth_header: authHeader || '',
    },
    { skipBusinessError: true }
  )
  if (!res.data?.success) {
    throw new Error(res.data?.message || 'Connection test failed')
  }
  return res.data?.data ?? { tools: [] }
}

export async function getEnabledPlugins(): Promise<EnabledPlugin[]> {
  const res = await api.get('/api/plugin/enabled')
  const body = res.data as ApiEnvelope<EnabledPlugin[]>
  return body?.data ?? []
}
