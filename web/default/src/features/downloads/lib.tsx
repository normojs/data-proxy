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
import {
  DOWNLOAD_ICON_KEYS,
  type DownloadIconKey,
  type DownloadItem,
} from './types'

export const DOWNLOAD_ICON_OPTIONS: Array<{
  value: DownloadIconKey
  label: string
}> = DOWNLOAD_ICON_KEYS.map((value) => {
  const labels: Record<DownloadIconKey, string> = {
    terminal: 'Terminal',
    code: 'Code',
    package: 'Package',
    bot: 'Agent',
    cloud: 'Cloud',
    desktop: 'Desktop',
    apple: 'Apple',
    windows: 'Windows',
    linux: 'Linux',
    download: 'Download',
  }

  return { value, label: labels[value] }
})

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

export function normalizeDownloadItem(
  item: Partial<DownloadItem>,
  fallbackId: number
): DownloadItem | null {
  const name = String(item.name ?? '').trim()
  const url = String(item.url ?? '').trim()
  if (!name || !url) return null

  const icon = DOWNLOAD_ICON_OPTIONS.some(
    (option) => option.value === item.icon
  )
    ? (item.icon as DownloadIconKey)
    : 'download'

  return {
    id: Number.isFinite(Number(item.id)) ? Number(item.id) : fallbackId,
    name,
    description: String(item.description ?? '').trim(),
    url,
    icon,
    customIconUrl: String(item.customIconUrl ?? '').trim(),
    openInNewTab:
      typeof item.openInNewTab === 'boolean' ? item.openInNewTab : true,
    enabled: typeof item.enabled === 'boolean' ? item.enabled : true,
  }
}

export function parseDownloadItems(data: unknown): DownloadItem[] {
  if (!data) return []

  let parsed = data
  if (typeof data === 'string') {
    try {
      parsed = JSON.parse(data)
    } catch {
      return []
    }
  }

  if (!Array.isArray(parsed)) return []

  return parsed.reduce<DownloadItem[]>((items, item, index) => {
    const normalized = normalizeDownloadItem(
      item as Partial<DownloadItem>,
      index + 1
    )
    if (normalized) items.push(normalized)
    return items
  }, [])
}

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
