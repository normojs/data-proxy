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
import {
  Code2,
  Database,
  FileText,
  Folder,
  Globe2,
  Search,
  Shield,
  Terminal,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { LongText } from '@/components/long-text'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import {
  getCallStatusLabel,
  getCallStatusVariant,
  getClientStatusLabel,
  getClientStatusVariant,
  getToolSourceLabel,
  getToolStatusLabel,
  getToolStatusVariant,
  getProxyServerStatusLabel,
  getProxyServerStatusVariant,
  getProxyToolStatusLabel,
  getProxyToolStatusVariant,
} from '../constants'

const categoryIconRules: Array<{ icon: LucideIcon; tokens: string[] }> = [
  { icon: Search, tokens: ['search', 'query', '检索', '搜索'] },
  { icon: Globe2, tokens: ['web', 'http', 'browser', 'url', '网络', '网页'] },
  { icon: FileText, tokens: ['file', 'doc', 'pdf', 'text', '文件', '文档'] },
  { icon: Database, tokens: ['data', 'db', 'sql', 'database', '数据'] },
  { icon: Code2, tokens: ['code', 'dev', 'api', '代码', '开发'] },
  { icon: Terminal, tokens: ['shell', 'terminal', 'cli', 'command', '终端'] },
  { icon: Shield, tokens: ['security', 'auth', 'safe', '安全', '权限'] },
]

function getCategoryIcon(category?: string): LucideIcon {
  const normalized = (category || '').trim().toLowerCase()
  if (!normalized) return Folder
  return (
    categoryIconRules.find((rule) =>
      rule.tokens.some((token) => normalized.includes(token))
    )?.icon ?? Folder
  )
}

export function IdCell(props: { value: number | string }) {
  return <TableId value={props.value} className='w-[70px]' />
}

export function TimestampCell(props: { value: number }) {
  return (
    <span className='text-muted-foreground whitespace-nowrap tabular-nums'>
      {formatTimestampToDate(props.value)}
    </span>
  )
}

export function DurationCell(props: { value: number }) {
  if (!props.value) {
    return <span className='text-muted-foreground'>-</span>
  }
  return <span className='tabular-nums'>{props.value} ms</span>
}

export function SizeCell(props: { value: number }) {
  if (!props.value) {
    return <span className='text-muted-foreground'>-</span>
  }
  return <span className='tabular-nums'>{props.value.toLocaleString()} B</span>
}

export function LongTextCell(props: { className?: string; value?: string }) {
  return (
    <LongText className={props.className ?? 'max-w-[220px]'}>
      {props.value || '-'}
    </LongText>
  )
}

type TraceCellItem = {
  label: string
  value?: number | string | null
}

function hasTraceValue(
  value: TraceCellItem['value']
): value is number | string {
  if (typeof value === 'number') return value > 0
  return typeof value === 'string' && value.trim() !== ''
}

export function TraceCell(props: {
  className?: string
  items: TraceCellItem[]
}) {
  const items = props.items.filter((item) => hasTraceValue(item.value))

  if (items.length === 0) {
    return <span className='text-muted-foreground'>-</span>
  }

  return (
    <div className={cn('flex min-w-[180px] flex-col gap-1', props.className)}>
      {items.map((item) => (
        <div key={item.label} className='flex min-w-0 items-center gap-1.5'>
          <span className='text-muted-foreground shrink-0 text-[10px] font-medium'>
            {item.label}
          </span>
          <LongText className='max-w-[180px] font-mono text-xs'>
            {String(item.value)}
          </LongText>
        </div>
      ))}
    </div>
  )
}

export function SettlementCell(props: {
  cost: number
  freeUsed: boolean
  quota: number
  settledAt: number
}) {
  const { t } = useTranslation()
  const settled = props.settledAt > 0
  const hasCharge = props.quota > 0 || props.cost > 0

  return (
    <div className='flex min-w-[130px] flex-col gap-1'>
      <span className='tabular-nums'>{formatQuota(props.quota)}</span>
      {props.cost > 0 && (
        <span className='text-muted-foreground text-xs tabular-nums'>
          {t('Cost')} {props.cost.toFixed(4)}
        </span>
      )}
      <div className='flex flex-wrap items-center gap-1'>
        {props.freeUsed && (
          <StatusBadge label={t('Free Used')} variant='info' copyable={false} />
        )}
        <StatusBadge
          label={t(settled ? 'Settled' : hasCharge ? 'Unsettled' : 'No Quota')}
          variant={settled ? 'success' : hasCharge ? 'warning' : 'neutral'}
          copyable={false}
        />
      </div>
    </div>
  )
}

export function ToolStatusBadge(props: { status: number }) {
  const { t } = useTranslation()
  return (
    <StatusBadge
      label={t(getToolStatusLabel(props.status))}
      variant={getToolStatusVariant(props.status)}
      copyable={false}
    />
  )
}

export function ToolSourceBadge(props: { source: string }) {
  const { t } = useTranslation()
  return (
    <StatusBadge
      label={t(getToolSourceLabel(props.source))}
      variant='info'
      copyable={false}
    />
  )
}

export function ToolCategoryBadge(props: { category?: string }) {
  const { t } = useTranslation()
  const label = props.category?.trim() || 'Uncategorized'
  return (
    <StatusBadge
      label={t(label)}
      icon={getCategoryIcon(label)}
      variant='neutral'
      showDot={false}
      copyable={false}
    />
  )
}

export function ProxyServerStatusBadge(props: { status: string }) {
  const { t } = useTranslation()
  return (
    <StatusBadge
      label={t(getProxyServerStatusLabel(props.status))}
      variant={getProxyServerStatusVariant(props.status)}
      copyable={false}
    />
  )
}

export function ProxyToolStatusBadge(props: { status: string }) {
  const { t } = useTranslation()
  return (
    <StatusBadge
      label={t(getProxyToolStatusLabel(props.status))}
      variant={getProxyToolStatusVariant(props.status)}
      copyable={false}
    />
  )
}

export function ClientStatusBadge(props: { online: boolean; status: number }) {
  const { t } = useTranslation()
  return (
    <StatusBadge
      label={t(getClientStatusLabel(props.status, props.online))}
      variant={getClientStatusVariant(props.status, props.online)}
      pulse={props.online}
      copyable={false}
    />
  )
}

export function CallStatusBadge(props: { status: string }) {
  const { t } = useTranslation()
  return (
    <StatusBadge
      label={t(getCallStatusLabel(props.status) || props.status || 'Unknown')}
      variant={getCallStatusVariant(props.status)}
      copyable={false}
    />
  )
}
