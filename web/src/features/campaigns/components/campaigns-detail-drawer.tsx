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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'

import {
  getCampaignParticipants,
  getCampaignRewards,
  getCampaignStats,
  resendCampaignRewardEmail,
} from '../api'
import {
  CAMPAIGN_REWARD_STATUS,
  CAMPAIGN_REWARD_STATUSES,
  CAMPAIGN_STATUSES,
  CAMPAIGN_TYPES,
} from '../constants'
import type { CampaignReward } from '../types'
import { useCampaigns } from './campaigns-provider'

const DETAIL_PAGE_SIZE = 10

type CampaignsDetailDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function CampaignsDetailDrawer({
  open,
  onOpenChange,
}: CampaignsDetailDrawerProps) {
  const { t } = useTranslation()
  const { currentRow } = useCampaigns()
  const campaignId = currentRow?.id ?? 0

  const [participantsPage, setParticipantsPage] = useState(1)
  const [rewardsPage, setRewardsPage] = useState(1)
  const [resendingId, setResendingId] = useState<number | null>(null)

  // The drawer stays mounted across campaigns, so reset pagination whenever
  // the viewed campaign changes; otherwise a later page of one campaign is
  // requested for the next.
  useEffect(() => {
    setParticipantsPage(1)
    setRewardsPage(1)
  }, [campaignId])

  const statsQuery = useQuery({
    queryKey: ['campaign-stats', campaignId],
    queryFn: () => getCampaignStats(campaignId),
    enabled: open && campaignId > 0,
  })

  const participantsQuery = useQuery({
    queryKey: ['campaign-participants', campaignId, participantsPage],
    queryFn: () =>
      getCampaignParticipants(campaignId, {
        p: participantsPage,
        page_size: DETAIL_PAGE_SIZE,
      }),
    enabled: open && campaignId > 0,
    placeholderData: (previousData) => previousData,
  })
  const participants = participantsQuery.data?.data?.items ?? []
  const participantsTotal = participantsQuery.data?.data?.total ?? 0
  const participantsTotalPages = Math.max(
    1,
    Math.ceil(participantsTotal / DETAIL_PAGE_SIZE)
  )

  const rewardsQuery = useQuery({
    queryKey: ['campaign-rewards', campaignId, rewardsPage],
    queryFn: () =>
      getCampaignRewards(campaignId, {
        p: rewardsPage,
        page_size: DETAIL_PAGE_SIZE,
      }),
    enabled: open && campaignId > 0,
    placeholderData: (previousData) => previousData,
  })
  const rewards = rewardsQuery.data?.data?.items ?? []
  const rewardsTotal = rewardsQuery.data?.data?.total ?? 0
  const rewardsTotalPages = Math.max(
    1,
    Math.ceil(rewardsTotal / DETAIL_PAGE_SIZE)
  )

  const stats = statsQuery.data?.data

  const typeConfig = currentRow ? CAMPAIGN_TYPES[currentRow.type] : undefined
  const statusConfig = currentRow
    ? CAMPAIGN_STATUSES[currentRow.status]
    : undefined

  const queryClient = useQueryClient()
  const resendMutation = useMutation({
    mutationFn: (rewardId: number) => resendCampaignRewardEmail(rewardId),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Reward email queued'))
        queryClient.invalidateQueries({
          queryKey: ['campaign-rewards', campaignId],
        })
      } else {
        toast.error(res.message || t('Failed to resend reward email'))
      }
    },
    onError: handleServerError,
    onSettled: () => setResendingId(null),
  })

  const handleResend = (reward: CampaignReward) => {
    setResendingId(reward.id)
    resendMutation.mutate(reward.id)
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[720px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{currentRow?.name ?? t('Campaign Details')}</SheetTitle>
          <SheetDescription>
            {t('Campaign stats, participants and rewards.')}
          </SheetDescription>
        </SheetHeader>

        <div className={sideDrawerFormClassName()}>
          {!currentRow ? (
            <div className='space-y-3'>
              <Skeleton className='h-6 w-48' />
              <Skeleton className='h-24 w-full' />
              <Skeleton className='h-40 w-full' />
            </div>
          ) : (
            <>
              <SideDrawerSection>
                <div className='flex flex-wrap items-center gap-2'>
                  {typeConfig && (
                    <StatusBadge
                      label={t(typeConfig.labelKey)}
                      variant='neutral'
                      copyable={false}
                    />
                  )}
                  {statusConfig && (
                    <StatusBadge
                      label={t(statusConfig.labelKey)}
                      variant={statusConfig.variant}
                      copyable={false}
                    />
                  )}
                </div>
                {currentRow.description && (
                  <p className='text-muted-foreground text-sm'>
                    {currentRow.description}
                  </p>
                )}
                <p className='text-muted-foreground font-mono text-sm'>
                  {formatTimestampToDate(currentRow.start_at)}
                  {' → '}
                  {currentRow.end_at
                    ? formatTimestampToDate(currentRow.end_at)
                    : t('Never expires')}
                </p>
              </SideDrawerSection>

              <SideDrawerSection>
                <SideDrawerSectionHeader title={t('Stats')} />
                <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
                  <Card>
                    <CardContent className='p-4'>
                      <div className='text-muted-foreground text-xs'>
                        {t('Participants')}
                      </div>
                      <div className='text-xl font-semibold tabular-nums'>
                        {stats?.participant_count ?? '-'}
                      </div>
                    </CardContent>
                  </Card>
                  <Card>
                    <CardContent className='p-4'>
                      <div className='text-muted-foreground text-xs'>
                        {t('Rewards')}
                      </div>
                      <div className='text-xl font-semibold tabular-nums'>
                        {stats?.reward_count ?? '-'}
                      </div>
                    </CardContent>
                  </Card>
                  <Card>
                    <CardContent className='p-4'>
                      <div className='text-muted-foreground text-xs'>
                        {t('Dispatched')}
                      </div>
                      <div className='text-xl font-semibold tabular-nums'>
                        {stats?.dispatched_count ?? '-'}
                      </div>
                    </CardContent>
                  </Card>
                  <Card>
                    <CardContent className='p-4'>
                      <div className='text-muted-foreground text-xs'>
                        {t('Quota Awarded')}
                      </div>
                      <div className='text-xl font-semibold tabular-nums'>
                        {stats ? formatQuota(stats.total_quota) : '-'}
                      </div>
                    </CardContent>
                  </Card>
                </div>
              </SideDrawerSection>

              <SideDrawerSection>
                <SideDrawerSectionHeader title={t('Participants')} />
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('User')}</TableHead>
                      <TableHead>{t('Event')}</TableHead>
                      <TableHead>{t('Time')}</TableHead>
                      <TableHead>{t('Extra')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {participants.length === 0 ? (
                      <TableRow>
                        <TableCell
                          colSpan={4}
                          className='text-muted-foreground text-center'
                        >
                          {participantsQuery.isLoading
                            ? t('Loading...')
                            : t('No participants yet')}
                        </TableCell>
                      </TableRow>
                    ) : (
                      participants.map((participant) => {
                        const eventConfig =
                          CAMPAIGN_TYPES[participant.event_type]
                        return (
                          <TableRow key={participant.id}>
                            <TableCell>
                              {participant.username || participant.user_id}
                            </TableCell>
                            <TableCell>
                              {eventConfig
                                ? t(eventConfig.labelKey)
                                : participant.event_type}
                            </TableCell>
                            <TableCell className='font-mono'>
                              {formatTimestampToDate(participant.event_at)}
                            </TableCell>
                            <TableCell>
                              {participant.extra_json ? (
                                <span className='text-muted-foreground block max-w-[220px] truncate font-mono text-xs'>
                                  {participant.extra_json}
                                </span>
                              ) : (
                                <span className='text-muted-foreground'>-</span>
                              )}
                            </TableCell>
                          </TableRow>
                        )
                      })
                    )}
                  </TableBody>
                </Table>
                <div className='flex items-center justify-between gap-2'>
                  <span className='text-muted-foreground text-xs tabular-nums'>
                    {t('Page {{page}} of {{totalPages}}', {
                      page: participantsPage,
                      totalPages: participantsTotalPages,
                    })}
                  </span>
                  <div className='flex gap-2'>
                    <Button
                      size='sm'
                      variant='outline'
                      disabled={participantsPage <= 1}
                      onClick={() => setParticipantsPage((p) => p - 1)}
                    >
                      {t('Previous')}
                    </Button>
                    <Button
                      size='sm'
                      variant='outline'
                      disabled={participantsPage >= participantsTotalPages}
                      onClick={() => setParticipantsPage((p) => p + 1)}
                    >
                      {t('Next')}
                    </Button>
                  </div>
                </div>
              </SideDrawerSection>

              <SideDrawerSection>
                <SideDrawerSectionHeader title={t('Rewards')} />
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('User')}</TableHead>
                      <TableHead>{t('Quota')}</TableHead>
                      <TableHead>{t('Status')}</TableHead>
                      <TableHead>{t('Dispatched')}</TableHead>
                      <TableHead>{t('Email Sent')}</TableHead>
                      <TableHead>{t('Email Error')}</TableHead>
                      <TableHead className='text-right'>
                        {t('Actions')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {rewards.length === 0 ? (
                      <TableRow>
                        <TableCell
                          colSpan={7}
                          className='text-muted-foreground text-center'
                        >
                          {rewardsQuery.isLoading
                            ? t('Loading...')
                            : t('No rewards yet')}
                        </TableCell>
                      </TableRow>
                    ) : (
                      rewards.map((reward) => {
                        const rewardStatus =
                          CAMPAIGN_REWARD_STATUSES[reward.status]
                        const canResend =
                          reward.status === CAMPAIGN_REWARD_STATUS.DISPATCHED &&
                          reward.redemption_id !== 0
                        return (
                          <TableRow key={reward.id}>
                            <TableCell>
                              {reward.username || reward.user_id}
                            </TableCell>
                            <TableCell className='tabular-nums'>
                              {formatQuota(reward.quota)}
                            </TableCell>
                            <TableCell>
                              {rewardStatus && (
                                <StatusBadge
                                  label={t(rewardStatus.labelKey)}
                                  variant={rewardStatus.variant}
                                  copyable={false}
                                  className='-ml-1.5'
                                />
                              )}
                            </TableCell>
                            <TableCell className='font-mono'>
                              {formatTimestampToDate(reward.dispatched_at)}
                            </TableCell>
                            <TableCell className='font-mono'>
                              {reward.email_sent_at
                                ? formatTimestampToDate(reward.email_sent_at)
                                : '-'}
                            </TableCell>
                            <TableCell>
                              {reward.email_error ? (
                                <Tooltip>
                                  <TooltipTrigger
                                    render={
                                      <span className='text-muted-foreground block max-w-[160px] truncate text-xs'>
                                        {reward.email_error}
                                      </span>
                                    }
                                  />
                                  <TooltipContent>
                                    <div className='max-w-xs text-xs break-words'>
                                      {reward.email_error}
                                    </div>
                                  </TooltipContent>
                                </Tooltip>
                              ) : (
                                <span className='text-muted-foreground'>-</span>
                              )}
                            </TableCell>
                            <TableCell className='text-right'>
                              <Button
                                size='sm'
                                variant='outline'
                                disabled={
                                  !canResend || resendingId === reward.id
                                }
                                onClick={() => handleResend(reward)}
                              >
                                {resendingId === reward.id
                                  ? t('Sending...')
                                  : t('Resend')}
                              </Button>
                            </TableCell>
                          </TableRow>
                        )
                      })
                    )}
                  </TableBody>
                </Table>
                <div className='flex items-center justify-between gap-2'>
                  <span className='text-muted-foreground text-xs tabular-nums'>
                    {t('Page {{page}} of {{totalPages}}', {
                      page: rewardsPage,
                      totalPages: rewardsTotalPages,
                    })}
                  </span>
                  <div className='flex gap-2'>
                    <Button
                      size='sm'
                      variant='outline'
                      disabled={rewardsPage <= 1}
                      onClick={() => setRewardsPage((p) => p - 1)}
                    >
                      {t('Previous')}
                    </Button>
                    <Button
                      size='sm'
                      variant='outline'
                      disabled={rewardsPage >= rewardsTotalPages}
                      onClick={() => setRewardsPage((p) => p + 1)}
                    >
                      {t('Next')}
                    </Button>
                  </div>
                </div>
              </SideDrawerSection>
            </>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}
