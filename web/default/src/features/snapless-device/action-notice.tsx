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
import { AlertCircle, AlertTriangle, ArrowRight, XCircle } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import type { SnaplessActionHints } from './api'

function actionTitle(severity?: string) {
  switch (severity) {
    case 'danger':
      return 'Action required'
    case 'warning':
      return 'Attention needed'
    default:
      return 'Notice'
  }
}

function actionToneClass(severity?: string) {
  switch (severity) {
    case 'danger':
      return 'border-destructive/30 bg-destructive/5 text-foreground *:[svg]:text-destructive'
    case 'warning':
      return 'border-warning/40 bg-warning/5 text-foreground *:[svg]:text-warning'
    default:
      return 'border-info/30 bg-info/5 text-foreground *:[svg]:text-info'
  }
}

function ActionIcon({
  severity,
  className,
}: {
  severity?: string
  className?: string
}) {
  switch (severity) {
    case 'danger':
      return <XCircle className={className} />
    case 'warning':
      return <AlertTriangle className={className} />
    default:
      return <AlertCircle className={className} />
  }
}

function ActionButtons({ actions }: { actions: SnaplessActionHints }) {
  const { t } = useTranslation()
  return (
    <div className='flex shrink-0 flex-wrap gap-2'>
      {actions.primary?.href && (
        <Button size='sm' render={<a href={actions.primary.href} />}>
          {t(actions.primary.label)}
          <ArrowRight data-icon='inline-end' />
        </Button>
      )}
      {actions.secondary?.href && (
        <Button
          variant='outline'
          size='sm'
          render={<a href={actions.secondary.href} />}
        >
          {t(actions.secondary.label)}
        </Button>
      )}
    </div>
  )
}

export function SnaplessActionNotice({
  actions,
  compact = false,
  className,
}: {
  actions?: SnaplessActionHints
  compact?: boolean
  className?: string
}) {
  const { t } = useTranslation()

  if (!actions?.reason) return null

  const toneClass = actionToneClass(actions.severity)

  if (compact) {
    return (
      <div
        className={cn(
          'flex flex-col gap-2 rounded-md border px-2.5 py-2 text-xs sm:flex-row sm:items-center sm:justify-between',
          toneClass,
          className
        )}
      >
        <div className='flex min-w-0 items-start gap-2'>
          <ActionIcon
            severity={actions.severity}
            className='mt-0.5 size-3.5 shrink-0'
          />
          <span className='min-w-0'>{t(actions.reason)}</span>
        </div>
        <ActionButtons actions={actions} />
      </div>
    )
  }

  return (
    <Alert className={cn(toneClass, className)}>
      <ActionIcon severity={actions.severity} className='size-4' />
      <AlertTitle>{t(actionTitle(actions.severity))}</AlertTitle>
      <AlertDescription className='mt-1 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
        <span>{t(actions.reason)}</span>
        <ActionButtons actions={actions} />
      </AlertDescription>
    </Alert>
  )
}
