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
export const CAMPAIGN_STATUS = {
  DRAFT: 1,
  ACTIVE: 2,
  PAUSED: 3,
  ENDED: 4,
} as const

export const CAMPAIGN_STATUSES: Record<
  number,
  {
    labelKey: string
    variant: 'neutral' | 'success' | 'warning' | 'danger' | 'info'
  }
> = {
  [CAMPAIGN_STATUS.DRAFT]: { labelKey: 'Draft', variant: 'neutral' },
  [CAMPAIGN_STATUS.ACTIVE]: { labelKey: 'Active', variant: 'success' },
  [CAMPAIGN_STATUS.PAUSED]: { labelKey: 'Paused', variant: 'warning' },
  [CAMPAIGN_STATUS.ENDED]: { labelKey: 'Ended', variant: 'info' },
}

export const CAMPAIGN_TYPE = {
  PHONE_FILLED: 'phone_filled',
  INVITATION: 'invitation',
} as const

export const CAMPAIGN_TYPES: Record<string, { labelKey: string }> = {
  [CAMPAIGN_TYPE.PHONE_FILLED]: { labelKey: 'Phone Filled' },
  [CAMPAIGN_TYPE.INVITATION]: { labelKey: 'Invitation' },
}

export const CAMPAIGN_REWARD_STATUS = {
  PENDING: 1,
  DISPATCHED: 2,
  FAILED: 3,
  CANCELLED: 4,
} as const

export const CAMPAIGN_REWARD_STATUSES: Record<
  number,
  {
    labelKey: string
    variant: 'neutral' | 'success' | 'warning' | 'danger' | 'info'
  }
> = {
  [CAMPAIGN_REWARD_STATUS.PENDING]: { labelKey: 'Pending', variant: 'neutral' },
  [CAMPAIGN_REWARD_STATUS.DISPATCHED]: {
    labelKey: 'Dispatched',
    variant: 'success',
  },
  [CAMPAIGN_REWARD_STATUS.FAILED]: { labelKey: 'Failed', variant: 'danger' },
  [CAMPAIGN_REWARD_STATUS.CANCELLED]: {
    labelKey: 'Cancelled',
    variant: 'warning',
  },
}

export const CAMPAIGN_CODE_COUNT_MAX = 1000

export const ERROR_MESSAGES = {
  LOAD_FAILED: 'Failed to load campaigns',
  SEARCH_FAILED: 'Failed to search campaigns',
} as const
