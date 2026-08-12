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

import type { OpsUserColumnMeta, OpsUsersListData } from './types'

interface ApiEnvelope<T> {
  success: boolean
  message: string
  data?: T
}

export async function getOpsUsers(params: {
  p: number
  page_size: number
}): Promise<ApiEnvelope<OpsUsersListData>> {
  const res = await api.get('/api/ops/user/', { params })
  return res.data
}

export async function searchOpsUsers(params: {
  keyword: string
  p: number
  page_size: number
}): Promise<ApiEnvelope<OpsUsersListData>> {
  const res = await api.get('/api/ops/user/search', { params })
  return res.data
}

export async function getOpsUserColumns(): Promise<
  ApiEnvelope<OpsUserColumnMeta[]>
> {
  const res = await api.get('/api/ops/user/columns')
  return res.data
}

export interface OpsUsersExportResult {
  blob: Blob
  filename?: string
}

export async function exportOpsUsers(payload: {
  ids?: number[]
  keyword?: string
}): Promise<OpsUsersExportResult> {
  const res = await api.post('/api/ops/user/export', payload, {
    responseType: 'blob',
    // Error responses are blobs here, so the axios error interceptor cannot
    // read the server message; let the caller surface a localized toast.
    skipErrorHandler: true,
  })
  const blob = res.data as Blob
  const disposition = res.headers['content-disposition'] as string | undefined
  const filename = disposition?.match(/filename="([^"]+)"/)?.[1]
  return { blob, filename }
}
