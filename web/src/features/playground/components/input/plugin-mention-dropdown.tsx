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
import { cn } from '@/lib/utils'

import type { EnabledPlugin } from '../../hooks/use-enabled-plugins'

interface PluginMentionDropdownProps {
  matches: EnabledPlugin[]
  activeIndex: number
  onSelect: (slug: string) => void
  onActiveIndexChange: (index: number) => void
}

export function PluginMentionDropdown({
  matches,
  activeIndex,
  onSelect,
  onActiveIndexChange,
}: PluginMentionDropdownProps) {
  if (matches.length === 0) return null

  return (
    <div className='bg-popover text-popover-foreground absolute bottom-full z-100 mb-1 w-full rounded-md border shadow-md'>
      <ul role='listbox' className='max-h-[240px] overflow-y-auto p-1'>
        {matches.map((plugin, index) => (
          <li
            key={plugin.slug}
            role='option'
            aria-selected={index === activeIndex}
            className={cn(
              'relative flex cursor-pointer flex-col gap-0.5 rounded-sm px-2 py-1.5 text-sm select-none',
              index === activeIndex && 'bg-accent text-accent-foreground'
            )}
            onMouseEnter={() => onActiveIndexChange(index)}
            onMouseDown={(event) => {
              event.preventDefault() // keep textarea focus
              onSelect(plugin.slug)
            }}
          >
            <span className='flex items-baseline gap-2'>
              <span className='font-medium'>{plugin.name}</span>
              <span className='text-muted-foreground text-xs'>
                @{plugin.slug}
              </span>
            </span>
            {plugin.description && (
              <span className='text-muted-foreground truncate text-xs'>
                {plugin.description}
              </span>
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}
