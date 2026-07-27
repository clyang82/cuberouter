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
import { Plus } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'

import {
  deletePlugin,
  getPlugins,
  refreshPluginSkill,
  updatePlugin,
} from './api'
import { PluginMutateDrawer } from './components/plugin-mutate-drawer'
import { PluginsTable } from './components/plugins-table'
import type { Plugin } from './types'

export function Plugins() {
  const { t } = useTranslation()
  const [plugins, setPlugins] = useState<Plugin[]>([])
  const [loading, setLoading] = useState(true)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editing, setEditing] = useState<Plugin | null>(null)
  const [togglingId, setTogglingId] = useState<number | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setPlugins(await getPlugins())
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const handleToggleEnabled = async (plugin: Plugin, enabled: boolean) => {
    setTogglingId(plugin.id)
    try {
      const response = await updatePlugin({ ...plugin, enabled })
      if (response.success) {
        setPlugins((previous) =>
          previous.map((item) =>
            item.id === plugin.id ? { ...item, enabled } : item
          )
        )
      }
    } finally {
      setTogglingId(null)
    }
  }

  const handleDelete = async (plugin: Plugin) => {
    const response = await deletePlugin(plugin.id)
    if (!response.success) {
      toast.error(response.message || t('Request failed'))
      throw new Error(response.message || 'delete failed')
    }
    toast.success(t('Plugin deleted'))
    await load()
  }

  const handleRefresh = async (plugin: Plugin) => {
    const response = await refreshPluginSkill(plugin.id)
    if (!response.success) {
      // Refresh failures carry the skill-fetch error in message; surface
      // them directly instead of the generic global toast.
      toast.error(response.message || t('Request failed'))
      return
    }
    toast.success(t('Skill refreshed'))
    await load()
  }

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('Plugins')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button
            onClick={() => {
              setEditing(null)
              setDrawerOpen(true)
            }}
          >
            <Plus data-icon='inline-start' />
            {t('New Plugin')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <PluginsTable
            plugins={plugins}
            loading={loading}
            togglingId={togglingId}
            onToggleEnabled={handleToggleEnabled}
            onEdit={(plugin) => {
              setEditing(plugin)
              setDrawerOpen(true)
            }}
            onDelete={handleDelete}
            onRefresh={handleRefresh}
          />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      {/* SectionPageLayout only renders its Title/Actions/Content slots, so
          the drawer must live outside it to stay mounted. */}
      <PluginMutateDrawer
        open={drawerOpen}
        plugin={editing}
        onOpenChange={setDrawerOpen}
        onSaved={load}
      />
    </>
  )
}
