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
import { Loader2, MoreHorizontal, Pencil, RefreshCw, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatTimestampToDate } from '@/lib/format'

import type { Plugin } from '../types'

type PluginsTableProps = {
  plugins: Plugin[]
  loading: boolean
  togglingId: number | null
  onToggleEnabled: (plugin: Plugin, enabled: boolean) => void
  onEdit: (plugin: Plugin) => void
  onDelete: (plugin: Plugin) => Promise<void>
  onRefresh: (plugin: Plugin) => Promise<void>
}

export function PluginsTable(props: PluginsTableProps) {
  const { t } = useTranslation()
  const {
    plugins,
    loading,
    togglingId,
    onToggleEnabled,
    onEdit,
    onDelete,
    onRefresh,
  } = props
  const [deleteTarget, setDeleteTarget] = useState<Plugin | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)
  const [refreshingId, setRefreshingId] = useState<number | null>(null)

  const handleConfirmDelete = async () => {
    if (!deleteTarget) return
    setIsDeleting(true)
    try {
      await onDelete(deleteTarget)
      setDeleteTarget(null)
    } finally {
      setIsDeleting(false)
    }
  }

  const handleRefresh = async (plugin: Plugin) => {
    setRefreshingId(plugin.id)
    try {
      await onRefresh(plugin)
    } finally {
      setRefreshingId(null)
    }
  }

  let body
  if (loading) {
    body = [0, 1, 2].map((row) => (
      <TableRow key={`plugin-skeleton-${row}`}>
        <TableCell colSpan={7}>
          <Skeleton className='h-8 w-full' />
        </TableCell>
      </TableRow>
    ))
  } else if (plugins.length === 0) {
    body = (
      <TableRow>
        <TableCell
          colSpan={7}
          className='text-muted-foreground h-24 text-center'
        >
          {t('No plugins yet. Create your first plugin to get started.')}
        </TableCell>
      </TableRow>
    )
  } else {
    body = plugins.map((plugin) => (
      <TableRow key={plugin.id}>
        <TableCell className='font-medium'>{plugin.name}</TableCell>
        <TableCell className='text-muted-foreground font-mono text-xs'>
          {plugin.slug}
        </TableCell>
        <TableCell
          className='max-w-[220px] truncate text-muted-foreground text-xs'
          title={plugin.mcp_url}
        >
          {plugin.mcp_url}
        </TableCell>
        <TableCell
          className='max-w-[200px] truncate text-muted-foreground text-xs'
          title={plugin.skill_source}
        >
          {plugin.skill_source || '—'}
        </TableCell>
        <TableCell>
          <Switch
            checked={plugin.enabled}
            disabled={togglingId === plugin.id}
            onCheckedChange={(checked) => onToggleEnabled(plugin, checked)}
            aria-label={t('Enabled')}
          />
        </TableCell>
        <TableCell className='text-muted-foreground text-xs whitespace-nowrap'>
          {plugin.skill_fetched_at
            ? formatTimestampToDate(plugin.skill_fetched_at)
            : '—'}
        </TableCell>
        <TableCell className='text-right'>
          <DropdownMenu>
            <DropdownMenuTrigger
              render={<Button variant='ghost' size='icon-sm' />}
            >
              <MoreHorizontal className='size-4' />
              <span className='sr-only'>{t('Open menu')}</span>
            </DropdownMenuTrigger>
            <DropdownMenuContent align='end' className='w-48'>
              <DropdownMenuItem onClick={() => onEdit(plugin)}>
                {t('Edit')}
                <DropdownMenuShortcut>
                  <Pencil size={16} />
                </DropdownMenuShortcut>
              </DropdownMenuItem>
              <DropdownMenuItem
                disabled={refreshingId === plugin.id || !plugin.skill_source}
                onClick={() => void handleRefresh(plugin)}
              >
                {t('Refresh Skill')}
                <DropdownMenuShortcut>
                  {refreshingId === plugin.id ? (
                    <Loader2 size={16} className='animate-spin' />
                  ) : (
                    <RefreshCw size={16} />
                  )}
                </DropdownMenuShortcut>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                className='text-destructive focus:text-destructive'
                onSelect={(event) => {
                  event.preventDefault()
                  setDeleteTarget(plugin)
                }}
              >
                {t('Delete')}
                <DropdownMenuShortcut>
                  <Trash2 size={16} />
                </DropdownMenuShortcut>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </TableCell>
      </TableRow>
    ))
  }

  return (
    <>
      <div className='overflow-x-auto rounded-md border'>
        <Table className='min-w-[900px]'>
          <TableHeader>
            <TableRow className='bg-muted/40 hover:bg-muted/40'>
              <TableHead className='h-9 px-4 text-xs'>{t('Name')}</TableHead>
              <TableHead className='h-9 text-xs'>{t('Slug')}</TableHead>
              <TableHead className='h-9 text-xs'>{t('MCP URL')}</TableHead>
              <TableHead className='h-9 text-xs'>{t('Skill Source')}</TableHead>
              <TableHead className='h-9 text-xs'>{t('Enabled')}</TableHead>
              <TableHead className='h-9 text-xs'>{t('Fetched At')}</TableHead>
              <TableHead className='h-9 w-[60px] text-xs' />
            </TableRow>
          </TableHeader>
          <TableBody>{body}</TableBody>
        </Table>
      </div>

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
        title={t('Delete Plugin')}
        desc={t(
          'Are you sure you want to delete plugin "{{name}}"? This action cannot be undone.',
          { name: deleteTarget?.name ?? '' }
        )}
        confirmText={t('Delete')}
        destructive
        isLoading={isDeleting}
        handleConfirm={() => void handleConfirmDelete()}
      />
    </>
  )
}
