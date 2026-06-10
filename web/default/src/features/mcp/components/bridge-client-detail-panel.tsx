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
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import {
  FileText,
  History,
  Pencil,
  RefreshCw,
  Trash2,
  Unplug,
  X,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatNumber } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { StatusBadge, StatusBadgeList } from '@/components/status-badge'
import {
  closeBridgeSession,
  deleteBridgeClient,
  getBridgeClient,
  getBridgeClientHealth,
  mcpQueryKeys,
  updateBridgeClient,
} from '../api'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type { BridgeClientUpdatePayload, BridgeSession } from '../types'
import { BridgeClientEditDialog } from './bridge-client-edit-dialog'
import {
  CallStatusBadge,
  ClientStatusBadge,
  TimestampCell,
} from './table-cells'

const route = getRouteApi('/_authenticated/mcp/$section')
const BRIDGE_HEALTH_WINDOW_SECONDS = 24 * 60 * 60

type BridgeClientDetailPanelProps = {
  clientId: string
  isAdmin?: boolean
  scope?: 'all'
  onClose: () => void
}

function formatRate(value: number): string {
  return `${formatNumber(value)}%`
}

function formatDuration(value: number): string {
  if (!value) return '-'
  return `${formatNumber(value)} ms`
}

function formatSize(value: number): string {
  if (!value) return '-'
  if (value < 1024) return `${formatNumber(value)} B`
  if (value < 1024 * 1024) return `${formatNumber(value / 1024)} KB`
  return `${formatNumber(value / 1024 / 1024)} MB`
}

function MetricCard(props: {
  label: string
  value: string
  detail?: string
  tone?: 'default' | 'success' | 'warning' | 'danger'
}) {
  const toneClass =
    props.tone === 'success'
      ? 'text-green-600 dark:text-green-400'
      : props.tone === 'warning'
        ? 'text-yellow-600 dark:text-yellow-400'
        : props.tone === 'danger'
          ? 'text-red-600 dark:text-red-400'
          : ''

  return (
    <div className='rounded-md border px-3 py-2'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className={`mt-1 text-xl font-semibold tabular-nums ${toneClass}`}>
        {props.value}
      </div>
      {props.detail && (
        <div className='text-muted-foreground mt-1 text-xs'>
          {props.detail}
        </div>
      )}
    </div>
  )
}

function SessionRow(props: {
  canManage?: boolean
  isClosing?: boolean
  session: BridgeSession
  onCloseSession?: (session: BridgeSession) => void
  onOpenAuditLogs: (sessionId: string) => void
  onOpenToolCalls: (sessionId: string) => void
}) {
  const { t } = useTranslation()
  const session = props.session

  return (
    <div className='grid gap-2 py-2 text-xs first:pt-0 last:pb-0 lg:grid-cols-[minmax(0,1fr)_auto]'>
      <div className='min-w-0'>
        <div className='flex min-w-0 flex-wrap items-center gap-2'>
          <StatusBadge
            label={t(session.status || 'Unknown')}
            variant={session.status === 'online' ? 'success' : 'neutral'}
            copyable={false}
          />
          <span className='max-w-[260px] truncate font-mono text-sm font-medium'>
            {session.session_id}
          </span>
        </div>
        <div className='text-muted-foreground mt-1 flex flex-wrap gap-x-3 gap-y-1 tabular-nums'>
          <span>
            {t('Connected At')}: <TimestampCell value={session.connected_at} />
          </span>
          <span>
            {t('Last Ping')}: <TimestampCell value={session.last_ping_at} />
          </span>
          {session.closed_at > 0 && (
            <span>
              {t('Closed At')}: <TimestampCell value={session.closed_at} />
            </span>
          )}
          {session.request_ip && (
            <span>
              {t('IP')}: {session.request_ip}
            </span>
          )}
        </div>
        {session.close_reason && (
          <div className='text-muted-foreground mt-1 truncate'>
            {session.close_reason}
          </div>
        )}
      </div>
      <div className='flex flex-wrap gap-1.5 lg:justify-end'>
        <Button
          type='button'
          variant='ghost'
          size='xs'
          onClick={() => props.onOpenAuditLogs(session.session_id)}
        >
          <FileText className='size-3' />
          {t('Open Audit Logs')}
        </Button>
        <Button
          type='button'
          variant='ghost'
          size='xs'
          onClick={() => props.onOpenToolCalls(session.session_id)}
        >
          <History className='size-3' />
          {t('Open Tool Calls')}
        </Button>
        {props.canManage && session.status === 'online' && (
          <Button
            type='button'
            variant='ghost'
            size='xs'
            disabled={props.isClosing}
            onClick={() => props.onCloseSession?.(session)}
          >
            <Unplug className='size-3' />
            {t('Close Session')}
          </Button>
        )}
      </div>
    </div>
  )
}

export function BridgeClientDetailPanel(props: BridgeClientDetailPanelProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const navigate = route.useNavigate()
  const [editorOpen, setEditorOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [closeSessionTarget, setCloseSessionTarget] =
    useState<BridgeSession | null>(null)
  const detailParams = useMemo(
    () => ({ scope: props.scope }),
    [props.scope]
  )
  const healthParams = useMemo(
    () => ({
      scope: props.scope,
      window_seconds: BRIDGE_HEALTH_WINDOW_SECONDS,
    }),
    [props.scope]
  )

  const {
    data: detail,
    error: detailError,
    isError: isDetailError,
    isFetching: isDetailFetching,
    refetch: refetchDetail,
  } = useQuery({
    queryKey: mcpQueryKeys.bridgeClientDetail(props.clientId, detailParams),
    queryFn: async () => {
      const result = await getBridgeClient(props.clientId, detailParams)
      if (!result.success || !result.data) {
        throw mcpQueryError(result.message, 'Failed to load bridge client')
      }
      return result.data
    },
  })

  const {
    data: health,
    error: healthError,
    isError: isHealthError,
    isFetching: isHealthFetching,
    refetch: refetchHealth,
  } = useQuery({
    queryKey: mcpQueryKeys.bridgeClientHealth(props.clientId, healthParams),
    queryFn: async () => {
      const result = await getBridgeClientHealth(props.clientId, healthParams)
      if (!result.success || !result.data) {
        throw mcpQueryError(
          result.message,
          'Failed to load bridge client health'
        )
      }
      return result.data
    },
  })

  useEffect(() => {
    if (isDetailError) {
      toast.error(
        mcpQueryErrorMessage(detailError, t('Failed to load bridge client'))
      )
    }
  }, [detailError, isDetailError, t])

  useEffect(() => {
    if (isHealthError) {
      toast.error(
        mcpQueryErrorMessage(
          healthError,
          t('Failed to load bridge client health')
        )
      )
    }
  }, [healthError, isHealthError, t])

  const invalidateBridgeClientQueries = () => {
    queryClient.invalidateQueries({ queryKey: mcpQueryKeys.bridgeClients() })
  }

  const updateMutation = useMutation({
    mutationFn: async (payload: BridgeClientUpdatePayload) =>
      updateBridgeClient(props.clientId, payload, detailParams),
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to update bridge client'))
        return
      }
      toast.success(t('Bridge client updated successfully'))
      invalidateBridgeClientQueries()
      setEditorOpen(false)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => deleteBridgeClient(props.clientId, detailParams),
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Failed to archive bridge client'))
        return
      }
      toast.success(t('Bridge client archived successfully'))
      invalidateBridgeClientQueries()
      setDeleteOpen(false)
      props.onClose()
    },
  })

  const closeSessionMutation = useMutation({
    mutationFn: (session: BridgeSession) =>
      closeBridgeSession(
        session.session_id,
        { reason: 'closed from dashboard' },
        detailParams
      ),
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Failed to close bridge session'))
        return
      }
      toast.success(t('Bridge session closed successfully'))
      invalidateBridgeClientQueries()
      setCloseSessionTarget(null)
    },
  })

  const client = detail?.client
  const sessions = health?.recent_sessions ?? detail?.recent_sessions ?? []
  const failures = (health?.calls.error ?? 0) + (health?.calls.timeout ?? 0)
  const isFetching =
    isDetailFetching ||
    isHealthFetching ||
    updateMutation.isPending ||
    deleteMutation.isPending ||
    closeSessionMutation.isPending

  const openAuditLogs = (extra?: {
    requestId?: string
    sessionId?: string
    status?: string
  }) => {
    void navigate({
      to: '/mcp/$section',
      params: { section: 'audit-logs' },
      search: (prev) => ({
        ...prev,
        auditPage: undefined,
        auditStartTime: Date.now() - BRIDGE_HEALTH_WINDOW_SECONDS * 1000,
        auditEndTime: undefined,
        auditStatus: extra?.status ? [extra.status] : undefined,
        requestId: extra?.requestId,
        clientId: props.clientId,
        sessionId: extra?.sessionId,
      }),
    })
  }

  const openToolCalls = (extra?: {
    requestId?: string
    sessionId?: string
    status?: string
    toolName?: string
  }) => {
    void navigate({
      to: '/mcp/$section',
      params: { section: 'tool-calls' },
      search: (prev) => ({
        ...prev,
        callsPage: undefined,
        callsStartTime: Date.now() - BRIDGE_HEALTH_WINDOW_SECONDS * 1000,
        callsEndTime: undefined,
        callStatus: extra?.status ? [extra.status] : undefined,
        requestId: extra?.requestId,
        sessionId: extra?.sessionId,
        targetClient: props.clientId,
        toolName: extra?.toolName ?? '',
      }),
    })
  }

  if (!client) {
    return (
      <div className='rounded-lg border px-4 py-3'>
        <div className='flex items-center justify-between gap-3'>
          <div>
            <div className='text-sm font-medium'>
              {t('Bridge Client Detail')}
            </div>
            <div className='text-muted-foreground mt-1 text-sm'>
              {t('Loading bridge client detail...')}
            </div>
          </div>
          <Button variant='ghost' size='icon-sm' onClick={props.onClose}>
            <X className='size-4' />
            <span className='sr-only'>{t('Close')}</span>
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className='rounded-lg border px-4 py-3'>
      <div className='flex flex-col gap-3 md:flex-row md:items-start md:justify-between'>
        <div className='min-w-0 space-y-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <span className='text-base font-semibold'>
              {client.name || client.client_id}
            </span>
            <ClientStatusBadge status={client.status} online={client.online} />
            {client.version && (
              <StatusBadge
                label={client.version}
                variant='neutral'
                copyable={false}
              />
            )}
            {client.platform && (
              <StatusBadge
                label={client.platform}
                autoColor={client.platform}
                copyable={false}
              />
            )}
          </div>
          <div className='text-muted-foreground flex flex-wrap gap-x-4 gap-y-1 text-xs'>
            <span className='font-mono'>{client.client_id}</span>
            <span>{client.workspace || '-'}</span>
            <span>
              {t('Last Seen')}: <TimestampCell value={client.last_seen_at} />
            </span>
          </div>
          {client.capabilities.length > 0 && (
            <StatusBadgeList
              items={client.capabilities}
              max={5}
              renderItem={(capability) => (
                <StatusBadge
                  label={capability}
                  autoColor={capability}
                  copyable={false}
                />
              )}
            />
          )}
        </div>

        <div className='flex shrink-0 flex-wrap gap-2'>
          <Button
            type='button'
            variant='outline'
            onClick={() => {
              void refetchDetail()
              void refetchHealth()
            }}
            disabled={isFetching}
          >
            <RefreshCw
              className={isFetching ? 'size-4 animate-spin' : 'size-4'}
            />
            {t('Refresh')}
          </Button>
          <Button
            type='button'
            variant='outline'
            onClick={() => openAuditLogs()}
          >
            <FileText className='size-4' />
            {t('Open Audit Logs')}
          </Button>
          {props.isAdmin && (
            <>
              <Button
                type='button'
                variant='outline'
                onClick={() => setEditorOpen(true)}
                disabled={isFetching}
              >
                <Pencil className='size-4' />
                {t('Edit')}
              </Button>
              {health?.online_session && (
                <Button
                  type='button'
                  variant='outline'
                  onClick={() =>
                    setCloseSessionTarget({
                      id: 0,
                      session_id: health.online_session?.session_id ?? '',
                      client_id: props.clientId,
                      user_id: health.online_session?.user_id ?? 0,
                      token_id: health.online_session?.token_id ?? 0,
                      request_ip: '',
                      user_agent: '',
                      status: 'online',
                      connected_at: health.online_session?.connected_at ?? 0,
                      last_ping_at: health.online_session?.last_seen_at ?? 0,
                      closed_at: 0,
                      close_reason: '',
                      created_at: health.online_session?.connected_at ?? 0,
                      updated_at: health.online_session?.last_seen_at ?? 0,
                    })
                  }
                  disabled={isFetching}
                >
                  <Unplug className='size-4' />
                  {t('Close Session')}
                </Button>
              )}
              <Button
                type='button'
                variant='ghost'
                onClick={() => setDeleteOpen(true)}
                disabled={isFetching}
              >
                <Trash2 className='size-4' />
                {t('Archive')}
              </Button>
            </>
          )}
          <Button type='button' variant='ghost' onClick={props.onClose}>
            <X className='size-4' />
            {t('Close')}
          </Button>
        </div>
      </div>

      <div className='mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
        <MetricCard
          label={t('24h Requests')}
          value={formatNumber(health?.calls.total_requests ?? 0)}
          detail={`${t('Success')}: ${formatNumber(health?.calls.success ?? 0)}`}
        />
        <MetricCard
          label={t('Request Success Rate')}
          value={formatRate(health?.calls.success_rate ?? 0)}
          detail={`${t('Errors')}: ${formatNumber(failures)}`}
          tone={failures > 0 ? 'warning' : 'success'}
        />
        <MetricCard
          label={t('Avg Duration')}
          value={formatDuration(health?.calls.avg_duration_ms ?? 0)}
          detail={`${t('Result Size')}: ${formatSize(health?.calls.result_size ?? 0)}`}
        />
        <MetricCard
          label={t('Session')}
          value={health?.online ? t('Online') : t('Offline')}
          detail={health?.online_session?.session_id || client.session_id || '-'}
          tone={health?.online ? 'success' : 'default'}
        />
      </div>

      {health?.online_session && (
        <div className='mt-4 rounded-md border px-3 py-2'>
          <div className='text-sm font-medium'>{t('Online Session')}</div>
          <div className='text-muted-foreground mt-2 grid gap-2 text-xs sm:grid-cols-2'>
            <span className='min-w-0 truncate font-mono'>
              {health.online_session.session_id}
            </span>
            <span>
              {t('Connected At')}:{' '}
              <TimestampCell value={health.online_session.connected_at} />
            </span>
            <span>
              {t('Last Seen')}:{' '}
              <TimestampCell value={health.online_session.last_seen_at} />
            </span>
            <span>
              {t('Token ID')}: {health.online_session.token_id || '-'}
            </span>
          </div>
        </div>
      )}

      <div className='mt-4 grid gap-3 lg:grid-cols-2'>
        <div className='rounded-md border px-3 py-2'>
          <div className='flex items-center justify-between gap-3'>
            <div className='text-sm font-medium'>{t('Recent Errors')}</div>
            <div className='text-muted-foreground text-xs'>
              {t('24h Window')}
            </div>
          </div>
          {!health || health.recent_errors.length === 0 ? (
            <div className='text-muted-foreground mt-2 text-sm'>
              {t('No recent errors')}
            </div>
          ) : (
            <div className='mt-2 space-y-2'>
              {health.recent_errors.slice(0, 5).map((error) => (
                <div
                  key={error.id}
                  className='grid gap-1 border-t pt-2 first:border-t-0 first:pt-0'
                >
                  <div className='flex min-w-0 flex-wrap items-center gap-2'>
                    <CallStatusBadge status={error.status} />
                    <span className='max-w-[220px] truncate font-mono text-xs'>
                      {error.request_id || '-'}
                    </span>
                    <TimestampCell value={error.created_at} />
                  </div>
                  <div className='text-muted-foreground truncate text-xs'>
                    {error.tool_name}
                    {error.error_message ? ` · ${error.error_message}` : ''}
                  </div>
                  <div className='flex flex-wrap gap-1.5'>
                    <Button
                      type='button'
                      variant='ghost'
                      size='xs'
                      onClick={() =>
                        openAuditLogs({
                          requestId: error.request_id,
                          sessionId: error.session_id,
                          status: error.status,
                        })
                      }
                    >
                      <FileText className='size-3' />
                      {t('Open Audit Logs')}
                    </Button>
                    <Button
                      type='button'
                      variant='ghost'
                      size='xs'
                      onClick={() =>
                        openToolCalls({
                          requestId: error.request_id,
                          sessionId: error.session_id,
                          status: error.status,
                          toolName: error.tool_name,
                        })
                      }
                    >
                      <History className='size-3' />
                      {t('Open Tool Calls')}
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className='rounded-md border px-3 py-2'>
          <div className='text-sm font-medium'>{t('Recent Sessions')}</div>
          {sessions.length === 0 ? (
            <div className='text-muted-foreground mt-2 text-sm'>
              {t('No recent sessions')}
            </div>
          ) : (
            <div className='mt-2 divide-y'>
              {sessions.map((session) => (
                <SessionRow
                  key={session.id || session.session_id}
                  session={session}
                  canManage={props.isAdmin}
                  isClosing={closeSessionMutation.isPending}
                  onCloseSession={setCloseSessionTarget}
                  onOpenAuditLogs={(sessionId) =>
                    openAuditLogs({ sessionId })
                  }
                  onOpenToolCalls={(sessionId) =>
                    openToolCalls({ sessionId })
                  }
                />
              ))}
            </div>
          )}
        </div>
      </div>
      {editorOpen && (
        <BridgeClientEditDialog
          key={client.client_id}
          open={editorOpen}
          client={client}
          isSaving={updateMutation.isPending}
          onOpenChange={setEditorOpen}
          onSubmit={(payload) => updateMutation.mutate(payload)}
        />
      )}
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t('Archive Bridge Client')}
        desc={t(
          'This will disconnect the current session and remove the client registration. The same client can appear again if it reconnects.'
        )}
        confirmText={t('Archive')}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() => deleteMutation.mutate()}
      />
      <ConfirmDialog
        open={closeSessionTarget != null}
        onOpenChange={(open) => !open && setCloseSessionTarget(null)}
        title={t('Close Bridge Session')}
        desc={t('This will disconnect the selected bridge session.')}
        confirmText={t('Close Session')}
        destructive
        isLoading={closeSessionMutation.isPending}
        handleConfirm={() => {
          if (closeSessionTarget) {
            closeSessionMutation.mutate(closeSessionTarget)
          }
        }}
      />
    </div>
  )
}
