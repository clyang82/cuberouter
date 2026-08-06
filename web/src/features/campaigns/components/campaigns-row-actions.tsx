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
import { useMutation } from '@tanstack/react-query'
import type { Row } from '@tanstack/react-table'
import { CircleStop, Edit, Eye, Pause, Power, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTableRowActionMenu } from '@/components/data-table/core/row-action-menu'
import { Button } from '@/components/ui/button'
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
} from '@/components/ui/dropdown-menu'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { handleServerError } from '@/lib/handle-server-error'

import { updateCampaignStatus } from '../api'
import { CAMPAIGN_STATUS } from '../constants'
import { campaignSchema } from '../types'
import { useCampaigns } from './campaigns-provider'

interface CampaignsRowActionsProps<TData> {
  row: Row<TData>
}

export function CampaignsRowActions<TData>({
  row,
}: CampaignsRowActionsProps<TData>) {
  const { t } = useTranslation()
  const campaign = campaignSchema.parse(row.original)
  const { setOpen, setCurrentRow, refresh } = useCampaigns()

  const statusMutation = useMutation({
    mutationFn: ({ status }: { status: number; labelKey: string }) =>
      updateCampaignStatus(campaign.id, status),
    onSuccess: (res, variables) => {
      if (res.success) {
        toast.success(
          t('Campaign {{action}}', { action: t(variables.labelKey) })
        )
        if (res.warning) {
          toast.warning(res.warning)
        }
        refresh()
      } else {
        toast.error(res.message || t('Failed to update campaign status'))
      }
    },
    onError: handleServerError,
  })

  const handleQuickStatus = (status: number, labelKey: string) => {
    statusMutation.mutate({ status, labelKey })
  }

  const isDraft = campaign.status === CAMPAIGN_STATUS.DRAFT
  const isActive = campaign.status === CAMPAIGN_STATUS.ACTIVE
  const isPaused = campaign.status === CAMPAIGN_STATUS.PAUSED
  const hasStatusActions = isDraft || isActive || isPaused

  return (
    <div className='-ml-1.5 flex items-center gap-1'>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon-sm'
              onClick={() => {
                setCurrentRow(campaign)
                setOpen('update')
              }}
              aria-label={t('Edit')}
            />
          }
        >
          <Edit />
        </TooltipTrigger>
        <TooltipContent>{t('Edit')}</TooltipContent>
      </Tooltip>

      <DataTableRowActionMenu ariaLabel={t('Open menu')} modal={false}>
        <DropdownMenuItem
          onClick={() => {
            setCurrentRow(campaign)
            setOpen('detail')
          }}
        >
          {t('View details')}
          <DropdownMenuShortcut>
            <Eye size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        {hasStatusActions && <DropdownMenuSeparator />}
        {(isDraft || isPaused) && (
          <DropdownMenuItem
            onClick={() =>
              handleQuickStatus(CAMPAIGN_STATUS.ACTIVE, 'Activated')
            }
          >
            {t('Activate')}
            <DropdownMenuShortcut>
              <Power size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        )}
        {isActive && (
          <DropdownMenuItem
            onClick={() => handleQuickStatus(CAMPAIGN_STATUS.PAUSED, 'Paused')}
          >
            {t('Pause')}
            <DropdownMenuShortcut>
              <Pause size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        )}
        {(isActive || isPaused) && (
          <DropdownMenuItem
            onClick={() => handleQuickStatus(CAMPAIGN_STATUS.ENDED, 'Ended')}
          >
            {t('End')}
            <DropdownMenuShortcut>
              <CircleStop size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem
          onClick={() => {
            setCurrentRow(campaign)
            setOpen('delete')
          }}
          className='text-destructive focus:text-destructive'
        >
          {t('Delete')}
          <DropdownMenuShortcut>
            <Trash2 size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      </DataTableRowActionMenu>
    </div>
  )
}
