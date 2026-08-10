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
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import { CAMPAIGN_STATUS, CAMPAIGN_TYPE } from '../../constants'
import type { Campaign } from '../../types'
import {
  buildCampaignConfigJson,
  buildCampaignPayload,
  campaignToFormDefaults,
  fromDatetimeLocalValue,
  parseCampaignConfig,
  toDatetimeLocalValue,
  type CampaignFormData,
} from '../campaign-form'

const baseForm: CampaignFormData = {
  name: 'Summer',
  description: 'desc',
  type: CAMPAIGN_TYPE.PHONE_FILLED,
  status: CAMPAIGN_STATUS.ACTIVE,
  start_at: '2026-08-01T10:00',
  end_at: '2026-09-01T10:00',
  quota: 200,
  redemption_name: 'SUMMER',
  expire_days: 30,
  max_participants: 500,
  max_rewards_per_user: 1,
  invitee_user_id: 0,
  invitee_username: '',
  code_count: 0,
}

describe('datetime-local conversion', () => {
  it('converts 0 to empty string and back', () => {
    assert.equal(toDatetimeLocalValue(0), '')
    assert.equal(fromDatetimeLocalValue(''), 0)
    assert.equal(fromDatetimeLocalValue('not-a-date'), 0)
  })

  it('round-trips minute-precision timestamps in local time', () => {
    const ts = 1754496000 // divisible by 60; timezone-independent round trip
    assert.equal(fromDatetimeLocalValue(toDatetimeLocalValue(ts)), ts)
  })
})

describe('parseCampaignConfig', () => {
  it('returns defaulted zero values for empty or invalid JSON', () => {
    for (const raw of ['', '{bogus', '{"quota":"nope"}']) {
      const config = parseCampaignConfig(raw)
      assert.equal(config.quota, 0)
      assert.equal(config.redemption_count, 0)
      assert.equal(config.code_count, 0)
      assert.equal(config.invitee_username, '')
    }
  })

  it('parses a full config', () => {
    const config = parseCampaignConfig(
      '{"quota":500,"redemption_name":"X","redemption_count":1,"max_participants":10,"max_rewards_per_user":2,"expire_days":7,"invitee_user_id":42,"invitee_username":"boss","code_count":100}'
    )
    assert.equal(config.quota, 500)
    assert.equal(config.invitee_user_id, 42)
    assert.equal(config.code_count, 100)
  })
})

describe('buildCampaignConfigJson', () => {
  it('forces redemption_count=1 and keeps invitee fields for invitation', () => {
    const json = buildCampaignConfigJson({
      ...baseForm,
      type: CAMPAIGN_TYPE.INVITATION,
      invitee_user_id: 42,
      invitee_username: 'boss',
      code_count: 250,
    })
    const config = JSON.parse(json)
    assert.equal(config.redemption_count, 1)
    assert.equal(config.invitee_user_id, 42)
    assert.equal(config.invitee_username, 'boss')
    assert.equal(config.code_count, 250)
  })

  it('zeroes invitee-only fields for phone_filled', () => {
    const json = buildCampaignConfigJson({
      ...baseForm,
      invitee_user_id: 42, // stale form state must not leak into the payload
      invitee_username: 'boss',
      code_count: 250,
    })
    const config = JSON.parse(json)
    assert.equal(config.invitee_user_id, 0)
    assert.equal(config.invitee_username, '')
    assert.equal(config.code_count, 0)
    assert.equal(config.redemption_count, 0)
  })
})

describe('campaignToFormDefaults', () => {
  it('returns create defaults without a campaign', () => {
    const defaults = campaignToFormDefaults(null)
    assert.equal(defaults.type, CAMPAIGN_TYPE.PHONE_FILLED)
    assert.equal(defaults.status, CAMPAIGN_STATUS.DRAFT)
    assert.equal(defaults.code_count, 100)
    assert.equal(defaults.start_at, '')
  })

  it('maps an invitation campaign back to form values', () => {
    const campaign: Campaign = {
      id: 7,
      name: 'Inv',
      description: 'd',
      type: 'invitation',
      status: CAMPAIGN_STATUS.ACTIVE,
      start_at: 1754496000,
      end_at: 0,
      config_json:
        '{"quota":300,"invitee_user_id":42,"invitee_username":"boss","code_count":77}',
      created_by: 1,
      created_at: 1,
      updated_at: 1,
    }
    const form = campaignToFormDefaults(campaign)
    assert.equal(form.type, CAMPAIGN_TYPE.INVITATION)
    assert.equal(form.quota, 300)
    assert.equal(form.invitee_user_id, 42)
    assert.equal(form.code_count, 77)
    assert.equal(
      form.end_at,
      '',
      'end_at 0 (never expires) maps to an empty input'
    )
    assert.equal(form.start_at, toDatetimeLocalValue(1754496000))
  })
})

describe('buildCampaignPayload', () => {
  it('converts datetime-local to unix seconds and embeds config_json', () => {
    const payload = buildCampaignPayload(baseForm, 12)
    assert.equal(payload.id, 12)
    assert.equal(payload.start_at, fromDatetimeLocalValue('2026-08-01T10:00'))
    assert.equal(payload.end_at, fromDatetimeLocalValue('2026-09-01T10:00'))
    assert.equal(JSON.parse(payload.config_json as string).quota, 200)
  })

  it('omits id for creates', () => {
    const payload = buildCampaignPayload(baseForm)
    assert.equal('id' in payload, false)
  })
})
