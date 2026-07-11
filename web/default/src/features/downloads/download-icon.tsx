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
import type { ElementType } from 'react'
import {
  Apple,
  Bot,
  Cloud,
  Code2,
  Download,
  Laptop,
  MonitorDown,
  Package,
  Terminal,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import type { DownloadIconKey, DownloadItem } from './types'

const iconMap = {
  terminal: Terminal,
  code: Code2,
  package: Package,
  bot: Bot,
  cloud: Cloud,
  desktop: MonitorDown,
  apple: Apple,
  windows: Laptop,
  linux: Terminal,
  download: Download,
} satisfies Record<DownloadIconKey, ElementType>

export function DownloadIcon({
  item,
  className,
}: {
  item: Pick<DownloadItem, 'name' | 'icon' | 'customIconUrl'>
  className?: string
}) {
  if (item.customIconUrl) {
    return (
      <img
        src={item.customIconUrl}
        alt=''
        className={cn('size-5 rounded-sm object-contain', className)}
        loading='lazy'
      />
    )
  }

  const Icon = iconMap[item.icon] ?? Download
  return <Icon className={cn('size-5', className)} aria-hidden />
}
