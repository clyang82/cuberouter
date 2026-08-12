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

import { SectionPageLayout } from '@/components/layout'
import { useDebounce, useMediaQuery } from '@/hooks'
import { handleServerError } from '@/lib/handle-server-error'

import { getOpsCampaigns, searchOpsCampaigns } from './api'
import { OpsCampaignsDetailDrawer } from './components/ops-campaigns-detail-drawer'
import { OpsCampaignsTable } from './components/ops-campaigns-table'
import type { Campaign } from './types'

export function OpsCampaigns() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')

  const [keyword, setKeyword] = useState('')
  const debouncedKeyword = useDebounce(keyword, 300)
  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: isMobile ? 10 : 20,
  })
  const [selectedCampaign, setSelectedCampaign] = useState<Campaign | null>(
    null
  )
  const [drawerOpen, setDrawerOpen] = useState(false)

  // A new keyword invalidates the current page; jump back to the first one.
  useEffect(() => {
    setPagination((prev) => ({ ...prev, pageIndex: 0 }))
  }, [debouncedKeyword])

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'ops-campaigns',
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
        ? await searchOpsCampaigns({ ...params, keyword: trimmed })
        : await getOpsCampaigns(params)

      if (!result.success) {
        handleServerError(result)
        throw new Error(
          result.message ||
            (trimmed ? 'Failed to search campaigns' : 'Failed to load campaigns')
        )
      }

      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const handleRowClick = (campaign: Campaign) => {
    setSelectedCampaign(campaign)
    setDrawerOpen(true)
  }

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('Ops Campaigns')}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <OpsCampaignsTable
            data={data?.items ?? []}
            isLoading={isLoading}
            isFetching={isFetching}
            totalCount={data?.total ?? 0}
            pagination={pagination}
            onPaginationChange={setPagination}
            globalFilter={keyword}
            onGlobalFilterChange={setKeyword}
            onRowClick={handleRowClick}
          />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <OpsCampaignsDetailDrawer
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
        campaign={selectedCampaign}
      />
    </>
  )
}
