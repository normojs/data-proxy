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
import { useState } from 'react'
import { ListFilter, Route } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { StatusBadge } from '@/components/status-badge'
import type { UsageLog } from '../../data/schema'
import { DetailsDialog } from '../dialogs/details-dialog'

interface RequestIdCellProps {
  log: UsageLog
  isAdmin: boolean
  onFilter: (requestId: string, subsiteId?: number) => void
}

function formatRequestId(value: string): string {
  if (!value) return ''
  if (value.length <= 16) return value
  return `${value.slice(0, 6)}...${value.slice(-6)}`
}

export function RequestIdCell({ log, isAdmin, onFilter }: RequestIdCellProps) {
  const { t } = useTranslation()
  const [traceOpen, setTraceOpen] = useState(false)
  const requestId = log.request_id

  if (!requestId) {
    return <span className='text-muted-foreground/50 text-xs'>-</span>
  }

  return (
    <>
      <div className='flex max-w-[210px] items-center gap-1'>
        <TooltipProvider delay={300}>
          <Tooltip>
            <TooltipTrigger render={<div className='min-w-0 flex-1' />}>
              <StatusBadge
                label={formatRequestId(requestId)}
                copyText={requestId}
                size='sm'
                showDot={false}
                className='border-border/60 bg-muted/30 text-foreground h-6 max-w-full gap-1.5 overflow-hidden rounded-md border px-2 py-0.5 font-mono'
              />
            </TooltipTrigger>
            <TooltipContent
              side='top'
              className='max-w-xs font-mono text-xs break-all'
            >
              {requestId}
            </TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  variant='ghost'
                  size='icon'
                  className='text-muted-foreground hover:text-foreground size-6 shrink-0'
                  aria-label={t('View request trace')}
                  onClick={(event) => {
                    event.stopPropagation()
                    setTraceOpen(true)
                  }}
                />
              }
            >
              <Route className='size-3.5' aria-hidden='true' />
            </TooltipTrigger>
            <TooltipContent side='top'>
              {t('View request trace')}
            </TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  variant='ghost'
                  size='icon'
                  className='text-muted-foreground hover:text-foreground size-6 shrink-0'
                  aria-label={t('Filter by Request ID')}
                  onClick={(event) => {
                    event.stopPropagation()
                    onFilter(requestId, log.subsite_id)
                  }}
                />
              }
            >
              <ListFilter className='size-3.5' aria-hidden='true' />
            </TooltipTrigger>
            <TooltipContent side='top'>
              {t('Filter by Request ID')}
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </div>
      <DetailsDialog
        log={log}
        isAdmin={isAdmin}
        open={traceOpen}
        onOpenChange={setTraceOpen}
      />
    </>
  )
}
