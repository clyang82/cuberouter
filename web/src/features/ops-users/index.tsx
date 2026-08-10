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
import { useQuery } from '@tanstack/react-query'
import type { PaginationState } from '@tanstack/react-table'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { useDebounce, useMediaQuery } from '@/hooks'

import { exportOpsUsers, getOpsUsers, searchOpsUsers } from './api'
import { OpsUsersTable } from './components/ops-users-table'

export function OpsUsers() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')

  const [keyword, setKeyword] = useState('')
  const debouncedKeyword = useDebounce(keyword, 300)
  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: isMobile ? 10 : 20,
  })
  const [isExporting, setIsExporting] = useState(false)

  // A new keyword invalidates the current page; jump back to the first one.
  useEffect(() => {
    setPagination((prev) => ({ ...prev, pageIndex: 0 }))
  }, [debouncedKeyword])

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'ops-users',
      pagination.pageIndex + 1,
      pagination.pageSize,
      debouncedKeyword,
    ],
    queryFn: async () => {
      const params = {
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      }
      const trimmed = debouncedKeyword.trim()
      const result = trimmed
        ? await searchOpsUsers({ ...params, keyword: trimmed })
        : await getOpsUsers(params)

      if (!result.success) {
        toast.error(
          result.message ||
            t(trimmed ? 'Failed to search invitees' : 'Failed to load invitees')
        )
        return { items: [], total: 0 }
      }

      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const handleExport = async (selectedIds: number[]) => {
    setIsExporting(true)
    try {
      const trimmed = debouncedKeyword.trim()
      let payload: { ids?: number[]; keyword?: string }
      if (trimmed) {
        payload = { keyword: trimmed }
      } else if (selectedIds.length > 0) {
        payload = { ids: selectedIds }
      } else {
        // An empty keyword means "all invitees" on the backend.
        payload = { keyword: '' }
      }
      const { blob, filename } = await exportOpsUsers(payload)

      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = filename || 'invitees.csv'
      link.click()
      URL.revokeObjectURL(url)
    } catch {
      toast.error(t('Failed to export invitees'))
    } finally {
      setIsExporting(false)
    }
  }

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>{t('Invite History')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <OpsUsersTable
          data={data?.items ?? []}
          isLoading={isLoading}
          isFetching={isFetching}
          totalCount={data?.total ?? 0}
          pagination={pagination}
          onPaginationChange={setPagination}
          globalFilter={keyword}
          onGlobalFilterChange={setKeyword}
          isExporting={isExporting}
          onExport={handleExport}
        />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
