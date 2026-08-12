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
import type {
  ColumnDef,
  OnChangeFn,
  PaginationState,
} from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { Button } from '@/components/ui/button'
import { formatTimestampToDate } from '@/lib/format'

import { CAMPAIGN_STATUSES, CAMPAIGN_TYPES } from '../../campaigns/constants'
import { parseCampaignConfig } from '../../campaigns/lib/campaign-form'
import type { Campaign } from '../types'

type OpsCampaignsTableProps = {
  data: Campaign[]
  isLoading: boolean
  isFetching: boolean
  totalCount: number
  pagination: PaginationState
  onPaginationChange: OnChangeFn<PaginationState>
  globalFilter: string
  onGlobalFilterChange: OnChangeFn<string>
  onRowClick: (campaign: Campaign) => void
}

export function OpsCampaignsTable({
  data,
  isLoading,
  isFetching,
  totalCount,
  pagination,
  onPaginationChange,
  globalFilter,
  onGlobalFilterChange,
  onRowClick,
}: OpsCampaignsTableProps) {
  const { t } = useTranslation()
  const columns = useOpsCampaignsColumns(onRowClick)

  const { table } = useDataTable({
    data,
    columns,
    globalFilter,
    onGlobalFilterChange,
    pagination,
    onPaginationChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount,
  })

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No campaigns found')}
      skeletonKeyPrefix='ops-campaigns-skeleton'
      applyHeaderSize
      toolbarProps={{
        searchPlaceholder: t('Search campaigns'),
      }}
    />
  )
}

function useOpsCampaignsColumns(
  onRowClick: (campaign: Campaign) => void
): ColumnDef<Campaign>[] {
  const { t } = useTranslation()

  return [
    {
      accessorKey: 'id',
      header: t('ID'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <TableId value={row.getValue('id') as number} className='w-[60px]' />
      ),
      size: 80,
    },
    {
      accessorKey: 'name',
      header: t('Name'),
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <Button
          variant='link'
          className='h-auto p-0 font-medium'
          onClick={() => onRowClick(row.original)}
        >
          {row.getValue('name') as string}
        </Button>
      ),
      size: 200,
    },
    {
      accessorKey: 'type',
      header: t('Type'),
      cell: ({ row }) => {
        const typeConfig = CAMPAIGN_TYPES[row.getValue('type') as string]
        if (!typeConfig) {
          return null
        }
        return (
          <StatusBadge
            label={t(typeConfig.labelKey)}
            variant='neutral'
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
      size: 130,
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      meta: { mobileBadge: true },
      cell: ({ row }) => {
        const statusConfig = CAMPAIGN_STATUSES[row.getValue('status') as number]
        if (!statusConfig) {
          return null
        }
        return (
          <StatusBadge
            label={t(statusConfig.labelKey)}
            variant={statusConfig.variant}
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
      size: 120,
    },
    {
      id: 'period',
      header: t('Period'),
      meta: { mobileHidden: true },
      cell: ({ row }) => {
        const campaign = row.original
        return (
          <div className='text-muted-foreground min-w-[180px] font-mono text-sm'>
            {formatTimestampToDate(campaign.start_at)}
            {' → '}
            {campaign.end_at
              ? formatTimestampToDate(campaign.end_at)
              : t('Never expires')}
          </div>
        )
      },
      size: 240,
    },
    {
      id: 'invitee',
      header: t('Invitee'),
      meta: { mobileHidden: true },
      cell: ({ row }) => {
        const config = parseCampaignConfig(row.original.config_json)
        const invitee =
          config.invitee_username ||
          (config.invitee_user_id > 0 ? String(config.invitee_user_id) : '')
        if (!invitee) {
          return <span className='text-muted-foreground'>-</span>
        }
        return <span>{invitee}</span>
      },
      size: 160,
    },
  ]
}
