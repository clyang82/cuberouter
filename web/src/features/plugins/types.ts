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
export interface Plugin {
  id: number
  name: string
  slug: string
  description: string
  enabled: boolean
  mcp_url: string
  auth_header?: string
  auth_token?: string
  skill_source: string
  skill_content: string
  skill_fetched_at: number
  created_time: number
  updated_time: number
}

export interface EnabledPlugin {
  slug: string
  name: string
  description: string
}

export interface PluginTestResult {
  tools: string[]
}
