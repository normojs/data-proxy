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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Play, RefreshCw, Save } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatNumber, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import {
  getMCPProxyHeartbeat,
  mcpQueryKeys,
  runMCPProxyHeartbeat,
  updateMCPProxyHeartbeat,
} from '../api'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type {
  MCPProxyHeartbeatRunResponse,
  MCPProxyHeartbeatSettings,
  MCPProxyHeartbeatStatus,
} from '../types'

type ProxyHeartbeatDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onViewRun: (run: MCPProxyHeartbeatRunResponse) => void
}

type ProxyHeartbeatForm = {
  enabled: boolean
  intervalSeconds: string
  limit: string
  activeWindowSeconds: string
  timeoutSeconds: string
}

const defaultForm: ProxyHeartbeatForm = {
  enabled: false,
  intervalSeconds: '300',
  limit: '50',
  activeWindowSeconds: '1800',
  timeoutSeconds: '10',
}

function buildForm(status?: MCPProxyHeartbeatStatus): ProxyHeartbeatForm {
  const settings = status?.settings
  if (!settings) return defaultForm
  return {
    enabled: settings.enabled,
    intervalSeconds: String(settings.interval_seconds),
    limit: String(settings.limit),
    activeWindowSeconds: String(settings.active_window_seconds),
    timeoutSeconds: String(settings.timeout_seconds),
  }
}

function clampInteger(
  value: string,
  fallback: number,
  min: number,
  max: number
): number {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return fallback
  return Math.min(max, Math.max(min, Math.trunc(parsed)))
}

function buildSettings(form: ProxyHeartbeatForm): MCPProxyHeartbeatSettings {
  return {
    enabled: form.enabled,
    interval_seconds: clampInteger(form.intervalSeconds, 300, 30, 24 * 60 * 60),
    limit: clampInteger(form.limit, 50, 1, 200),
    active_window_seconds: clampInteger(
      form.activeWindowSeconds,
      1800,
      0,
      7 * 24 * 60 * 60
    ),
    timeout_seconds: clampInteger(form.timeoutSeconds, 10, 2, 120),
  }
}

function heartbeatStatusVariant(status?: string): StatusVariant {
  switch (status) {
    case 'success':
      return 'success'
    case 'partial':
    case 'running':
      return 'warning'
    case 'failed':
    case 'error':
      return 'danger'
    default:
      return 'neutral'
  }
}

function MetricTile(props: { label: string; value: string }) {
  return (
    <div className='bg-muted/20 rounded-lg border px-3 py-2'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className='mt-1 font-mono text-sm font-medium tabular-nums'>
        {props.value}
      </div>
    </div>
  )
}

export function ProxyHeartbeatDialog(props: ProxyHeartbeatDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [draftForm, setDraftForm] = useState<ProxyHeartbeatForm | null>(null)

  const {
    data: status,
    error,
    isError,
    isFetching,
    isLoading,
    refetch,
  } = useQuery({
    queryKey: mcpQueryKeys.proxyHeartbeat(),
    queryFn: async () => {
      const result = await getMCPProxyHeartbeat()
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load MCP heartbeat')
      }
      return result.data
    },
    enabled: props.open,
  })

  useEffect(() => {
    if (!isError) return
    toast.error(mcpQueryErrorMessage(error, t('Failed to load MCP heartbeat')))
  }, [error, isError, t])

  const saveMutation = useMutation({
    mutationFn: () => updateMCPProxyHeartbeat(buildSettings(form)),
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to save MCP heartbeat settings'))
        return
      }
      toast.success(t('MCP heartbeat settings saved'))
      setDraftForm(buildForm(res.data))
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.proxyHeartbeat() })
    },
  })

  const runMutation = useMutation({
    mutationFn: runMCPProxyHeartbeat,
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to run MCP heartbeat'))
        return
      }
      toast.success(t('MCP heartbeat completed'))
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.proxyHeartbeat() })
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.proxyServers() })
      props.onViewRun(res.data)
    },
  })

  const busy =
    isLoading ||
    saveMutation.isPending ||
    runMutation.isPending ||
    !!status?.running
  const form = draftForm ?? buildForm(status)
  const lastRun = status?.last_run
  const lastRunAt = status?.last_run_at
    ? formatTimestampToDate(status.last_run_at)
    : '-'

  return (
    <Dialog
      open={props.open}
      onOpenChange={(open) => {
        if (!open) setDraftForm(null)
        props.onOpenChange(open)
      }}
    >
      <DialogContent className='max-w-[calc(100%-2rem)] sm:max-w-3xl'>
        <DialogHeader>
          <div className='flex flex-wrap items-start justify-between gap-3 pr-8'>
            <div className='min-w-0'>
              <DialogTitle>{t('MCP Proxy Heartbeat')}</DialogTitle>
              <DialogDescription>
                {t(
                  'Keep observable proxy sessions warm and detect stale links.'
                )}
              </DialogDescription>
            </div>
            <div className='flex flex-wrap items-center gap-2'>
              <StatusBadge
                label={t(form.enabled ? 'Enabled' : 'Disabled')}
                variant={form.enabled ? 'success' : 'neutral'}
                copyable={false}
              />
              {status?.running && (
                <StatusBadge
                  label={t('Running')}
                  variant='warning'
                  pulse
                  copyable={false}
                />
              )}
            </div>
          </div>
        </DialogHeader>

        <div className='grid gap-3'>
          <div className='grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]'>
            <Label className='bg-muted/20 min-h-10 rounded-lg border px-3 py-2 text-sm font-normal'>
              <Switch
                size='sm'
                checked={form.enabled}
                disabled={isLoading || saveMutation.isPending}
                onCheckedChange={(checked) =>
                  setDraftForm({
                    ...form,
                    enabled: !!checked,
                  })
                }
              />
              <span>{t('Enable Heartbeat')}</span>
            </Label>
            <Button
              type='button'
              variant='outline'
              onClick={() => {
                setDraftForm(null)
                void refetch()
              }}
              disabled={isFetching}
              className={cn(isFetching && 'opacity-80')}
            >
              <RefreshCw
                className={cn('size-4', isFetching && 'animate-spin')}
              />
              {t('Refresh')}
            </Button>
          </div>

          <div className='grid gap-3 sm:grid-cols-2'>
            <Label className='flex-col items-start gap-1.5 text-xs font-normal'>
              <span className='text-muted-foreground'>
                {t('Interval Seconds')}
              </span>
              <Input
                type='number'
                min={30}
                max={86400}
                value={form.intervalSeconds}
                disabled={isLoading || saveMutation.isPending}
                onChange={(event) =>
                  setDraftForm({
                    ...form,
                    intervalSeconds: event.target.value,
                  })
                }
              />
            </Label>
            <Label className='flex-col items-start gap-1.5 text-xs font-normal'>
              <span className='text-muted-foreground'>{t('Batch Size')}</span>
              <Input
                type='number'
                min={1}
                max={200}
                value={form.limit}
                disabled={isLoading || saveMutation.isPending}
                onChange={(event) =>
                  setDraftForm({
                    ...form,
                    limit: event.target.value,
                  })
                }
              />
            </Label>
            <Label className='flex-col items-start gap-1.5 text-xs font-normal'>
              <span className='text-muted-foreground'>
                {t('Active Window Seconds')}
              </span>
              <Input
                type='number'
                min={0}
                max={604800}
                value={form.activeWindowSeconds}
                disabled={isLoading || saveMutation.isPending}
                onChange={(event) =>
                  setDraftForm({
                    ...form,
                    activeWindowSeconds: event.target.value,
                  })
                }
              />
            </Label>
            <Label className='flex-col items-start gap-1.5 text-xs font-normal'>
              <span className='text-muted-foreground'>
                {t('Timeout Seconds')}
              </span>
              <Input
                type='number'
                min={2}
                max={120}
                value={form.timeoutSeconds}
                disabled={isLoading || saveMutation.isPending}
                onChange={(event) =>
                  setDraftForm({
                    ...form,
                    timeoutSeconds: event.target.value,
                  })
                }
              />
            </Label>
          </div>

          <div className='rounded-lg border px-3 py-2'>
            <div className='flex flex-wrap items-start justify-between gap-3'>
              <div className='min-w-0'>
                <div className='flex flex-wrap items-center gap-2 text-sm font-medium'>
                  <span>{t('Last Run')}</span>
                  {status?.last_run_status && (
                    <StatusBadge
                      label={t(status.last_run_status)}
                      variant={heartbeatStatusVariant(status.last_run_status)}
                      copyable={false}
                    />
                  )}
                </div>
                <div className='text-muted-foreground mt-1 text-xs'>
                  {lastRunAt}
                  {status?.last_run_message ? (
                    <span>
                      {' · '}
                      {status.last_run_message}
                    </span>
                  ) : null}
                </div>
              </div>
              {lastRun && (
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => props.onViewRun(lastRun)}
                >
                  {t('View Result')}
                </Button>
              )}
            </div>
            <div className='mt-3 grid grid-cols-2 gap-2 sm:grid-cols-5'>
              <MetricTile
                label={t('Scanned')}
                value={formatNumber(lastRun?.scanned_count ?? 0)}
              />
              <MetricTile
                label={t('Pinged')}
                value={formatNumber(lastRun?.pinged_count ?? 0)}
              />
              <MetricTile
                label={t('Success')}
                value={formatNumber(lastRun?.success_count ?? 0)}
              />
              <MetricTile
                label={t('Errors')}
                value={formatNumber(lastRun?.error_count ?? 0)}
              />
              <MetricTile
                label={t('Skipped')}
                value={formatNumber(lastRun?.skipped_count ?? 0)}
              />
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => saveMutation.mutate()}
            disabled={isLoading || saveMutation.isPending}
          >
            {saveMutation.isPending ? (
              <RefreshCw className='size-4 animate-spin' />
            ) : (
              <Save className='size-4' />
            )}
            {t('Save')}
          </Button>
          <Button
            type='button'
            onClick={() => runMutation.mutate()}
            disabled={busy}
          >
            {runMutation.isPending || status?.running ? (
              <RefreshCw className='size-4 animate-spin' />
            ) : (
              <Play className='size-4' />
            )}
            {t('Run Now')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
