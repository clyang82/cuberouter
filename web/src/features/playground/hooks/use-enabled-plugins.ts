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
import { useEffect, useState } from 'react'

import { getEnabledPlugins } from '../api'

export interface EnabledPlugin {
  slug: string
  name: string
  description: string
}

let cache: EnabledPlugin[] | null = null

export function useEnabledPlugins() {
  const [plugins, setPlugins] = useState<EnabledPlugin[]>(cache ?? [])

  useEffect(() => {
    if (cache) return
    let cancelled = false
    void getEnabledPlugins()
      .then((list) => {
        cache = list
        if (!cancelled) setPlugins(list)
      })
      .catch(() => {
        // Leave cache null so a later mount retries the fetch.
      })
    return () => {
      cancelled = true
    }
  }, [])

  return { plugins }
}
