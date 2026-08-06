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
import { CampaignsDeleteDialog } from './campaigns-delete-dialog'
import { CampaignsDetailDrawer } from './campaigns-detail-drawer'
import { CampaignsMutateDrawer } from './campaigns-mutate-drawer'
import { useCampaigns } from './campaigns-provider'

export function CampaignsDialogs() {
  const { open, setOpen, currentRow } = useCampaigns()
  const isUpdate = open === 'update'

  return (
    <>
      <CampaignsMutateDrawer
        open={open === 'create' || isUpdate}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        currentRow={isUpdate ? currentRow || undefined : undefined}
      />
      <CampaignsDetailDrawer
        open={open === 'detail'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
      />
      <CampaignsDeleteDialog />
    </>
  )
}
